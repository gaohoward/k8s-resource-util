package k8sservice

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"gaohoward.tools/k8s/resutil/pkg/common"
	"gaohoward.tools/k8s/resutil/pkg/config"
	"gaohoward.tools/k8s/resutil/pkg/logs"
	"gioui.org/widget"
	"github.com/google/uuid"
	"go.uber.org/zap"
	yamlv3 "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/openapi/cached"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubectl/pkg/explain"
	ktlexplain "k8s.io/kubectl/pkg/explain/v2"
)

var k8sClient *K8sClient

var NoApiResourceInfo = common.ApiResourceInfo{}

func (k *K8sClient) GetCRDFor(resEntry *common.ApiResourceEntry) (string, error) {
	crd, err := GetCRDFor(resEntry, k.config, k.generator)
	if err != nil {
		return "", err
	}
	return crd, nil
}

func GetCRDFor(resEntry *common.ApiResourceEntry, k8sConfig *rest.Config, generator ktlexplain.Generator) (string, error) {
	if k8sConfig == nil {
		return "", fmt.Errorf("no rest client configured. Did you start the cluster?")
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(k8sConfig)
	if err != nil {
		logger.Error("error creating k8s client", zap.Error(err))
		return "", err
	}

	cachedClient, err := common.GetCachedDiscoveryClient(k8sConfig)
	if err != nil {
		logger.Error("error creating cached client", zap.Error(err))
		return "", err
	}

	v3client := cached.NewClient(discoveryClient.OpenAPIV3())
	// v2client := cached.NewClient(discoveryClient.WithLegacy().OpenAPIV3())

	v3paths, err := v3client.Paths()
	if err != nil {
		return "", err
	}

	var resourcePath string = resEntry.GetApiPath()

	gv, exists := v3paths[resourcePath]
	if !exists {
		return "", fmt.Errorf("couldn't found path for %s", resourcePath)
	}

	openAPISchemaBytes, err := gv.Schema(runtime.ContentTypeJSON)
	if err != nil {
		logger.Error("error getting schema", zap.Error(err))
		return "", err
	}

	var parsedV3Schema map[string]any
	if err := json.Unmarshal(openAPISchemaBytes, &parsedV3Schema); err != nil {
		return "", fmt.Errorf("error unmarshaling schema")
	}

	gvr, fieldsPath, err := GetGVR(cachedClient, resEntry)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)

	err = generator.Render("plaintext", parsedV3Schema, *gvr, fieldsPath, true, buf)

	if err != nil {
		return "", fmt.Errorf("error render %v", err)
	}
	fullSpec := buf.String()

	if len(fullSpec) < common.MAX_TEXT_SIZE {
		return fullSpec, nil
	}
	return fullSpec[:common.MAX_TEXT_SIZE] + "\n...(truncated)", nil
}

type K8sClient struct {
	lock            sync.RWMutex
	config          *rest.Config
	discoveryClient *discovery.DiscoveryClient
	mapper          *restmapper.DeferredDiscoveryRESTMapper
	dynClient       *dynamic.DynamicClient
	setupErr        string
	allRes          *common.ApiResourceInfo
	clusterInfo     *common.ClusterInfo
	generator       ktlexplain.Generator
	//	client    *rest.Config

}

func (k *K8sClient) GetClusterName() string {
	return k.clusterInfo.Id
}

func (k *K8sClient) GetClusterInfo() *common.ClusterInfo {
	return k.clusterInfo
}

func (k *K8sClient) FetchAllApiResources(force bool) *common.ApiResourceInfo {
	k.lock.Lock()
	defer k.lock.Unlock()

	fetched := false

	if (force || k.allRes == nil) && k.IsValid() {
		apiResourceList, err := k.discoveryClient.ServerPreferredResources()
		if err != nil {
			logger.Error("Error fetching API resources", zap.Error(err))
		} else {
			fetched = true
			k.allRes = &common.ApiResourceInfo{
				ResList: make([]*v1.APIResourceList, 0),
				ResMap:  make(map[string]*common.ApiResourceEntry, 0),
			}
			if len(k.allRes.ResList) > 0 {
				k.allRes.ResList = make([]*v1.APIResourceList, 0)
			}
			k.allRes.ResList = append(k.allRes.ResList, apiResourceList...)
			//fill the map
			if len(k.allRes.ResList) > 0 {
				for _, resList := range k.allRes.ResList {
					for _, res := range resList.APIResources {
						key := resList.GroupVersion + "/" + res.Name
						entry := &common.ApiResourceEntry{
							ApiVer: key,
							Gv:     resList.GroupVersion,
							ApiRes: &res,
						}
						if sch, err := k.GetCRDFor(entry); err == nil {
							entry.Schema = sch
						} else {
							entry.Schema = err.Error()
						}
						k.allRes.ResMap[key] = entry
					}
				}
			}
		}
	}

	if fetched {
		k.allRes.Cached = false
	} else if k.allRes != nil {
		k.allRes.Cached = true
	}

	return k.allRes
}

func (k *K8sClient) K8sObjectToYaml(obj runtime.Object) string {
	decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	writer := bytes.NewBufferString("")
	err := decoder.Encode(obj, writer)
	if err != nil {
		return fmt.Sprintf("Error encoding object to YAML: %v", err)
	}
	return writer.String()
}

func (k *K8sClient) DeployResource(res *common.ResourceInstanceAction, targetNs string) (types.NamespacedName, error) {

	appLogger := logs.GetLogger(logs.IN_APP_LOGGER_NAME)

	finalNamespace := types.NamespacedName{
		Name:      res.GetName(),
		Namespace: "",
	}

	if !k.IsValid() {
		appLogger.Warn("k8s client is not set updid you have a cluster up and running?")
		return finalNamespace, fmt.Errorf("no rest client")
	}

	obj := &unstructured.Unstructured{}
	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, gvk, err := dec.Decode([]byte(res.Instance.GetCR()), nil, obj)

	if err != nil {
		return finalNamespace, err
	}

	mapping, err := k.RetrieveMapping(gvk.GroupKind(), gvk.Version, true)

	if err != nil {
		logger.Debug("failed to get mapping", zap.String("err", err.Error()))
		return finalNamespace, err
	}
	if obj.GetNamespace() == "" {
		if targetNs != "" {
			obj.SetNamespace(targetNs)
		} else {
			obj.SetNamespace(res.GetDefaultNamespace())
		}
	}

	//update deployDetail
	finalNamespace.Namespace = obj.GetNamespace()

	var dr dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		// namespaced resources should specify the namespace
		dr = k.dynClient.Resource(mapping.Resource).Namespace(obj.GetNamespace())
	} else {
		// for cluster-wide resources
		dr = k.dynClient.Resource(mapping.Resource)
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return finalNamespace, err
	}

	switch res.GetAction() {
	case common.Create:
		appLogger.Info("CREATE resource", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()))
		if _, err := dr.Apply(context.TODO(), obj.GetName(), obj, v1.ApplyOptions{FieldManager: common.APP_NAME}); err != nil {
			appLogger.Error("Failed to create resource", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()))
			logger.Error("Failed to Create resource", zap.Error(err))
			return finalNamespace, err
		}
		appLogger.Info("Resource created successfully", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()))
	case common.Delete:
		appLogger.Info("DELETE resource", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()))
		if err := dr.Delete(context.TODO(), obj.GetName(), v1.DeleteOptions{}); err != nil {
			appLogger.Error("Failed to delete resource", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()), zap.Error(err))
			logger.Error("Failed to delete resource", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()), zap.Error(err))
			return finalNamespace, err
		}
		appLogger.Info("Resource deleted successfully", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()))
	case common.Update:
		appLogger.Info("UPDATE resource", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()))
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			// ApplyPatchType means server side apply
			if _, err := dr.Patch(context.TODO(), obj.GetName(), types.ApplyPatchType, data, v1.PatchOptions{FieldManager: common.APP_NAME}); err != nil {
				appLogger.Error("Failed to update resource", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()))
				logger.Error("Failed to update resource", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()), zap.Error(err))
				return err
			}
			appLogger.Info("Resource updated successfully", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()))
			return nil
		})
		if err != nil {
			return finalNamespace, err
		}
	}
	return finalNamespace, nil
}

func (k *K8sClient) NewRestMapper() {
	k.mapper = restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(k.discoveryClient))
}

func (k *K8sClient) RetrieveMapping(kind schema.GroupKind, version string, retry bool) (*meta.RESTMapping, error) {
	mapping, err := k.mapper.RESTMapping(kind, version)
	if err != nil && retry {
		logger.Info("Retry retrieving mapping", zap.String("err", err.Error()))
		k.NewRestMapper()
		return k.RetrieveMapping(kind, version, false)
	}
	return mapping, err
}

func (k *K8sClient) IsValid() bool {
	return k.setupErr == ""
}

func (k *K8sClient) SetupClients() {
	if k.config != nil {
		if dc, err := discovery.NewDiscoveryClientForConfig(k.config); err == nil {
			k.discoveryClient = dc
			k.NewRestMapper()
		} else {
			k.setupErr = err.Error()
		}
		if dyn, err := dynamic.NewForConfig(k.config); err == nil {
			k.dynClient = dyn
		} else {
			k.setupErr = err.Error()
		}

		key := k.config.Host + k.config.Username + k.config.CertFile

		clientId := fmt.Sprintf("%x", sha256.Sum256([]byte(key)))

		k.clusterInfo = &common.ClusterInfo{
			Host: k.config.Host,
			Id:   clientId,
		}
	} else {
		k.clusterInfo = &common.ClusterInfo{}
		k.setupErr = "No rest config"
	}

	k.generator = ktlexplain.NewGenerator()
	if err := registerBuiltinTemplates(k.generator); err != nil {
		logger.Warn("Error registing template", zap.Error(err))
	}
}

func (k *K8sClient) GetPodContainers(podRaw *unstructured.Unstructured) ([]string, error) {
	result := make([]string, 0)
	if k.IsValid() {
		pod := &corev1.Pod{}
		if runtime.DefaultUnstructuredConverter == nil {
			return nil, fmt.Errorf("no converter")
		}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(podRaw.Object, &pod)
		if err != nil {
			return nil, err
		}
		if len(pod.Spec.Containers) > 0 {
			for _, c := range pod.Spec.Containers {
				result = append(result, c.Name)
			}
		}
		if len(pod.Spec.InitContainers) > 0 {
			for _, ic := range pod.Spec.InitContainers {
				result = append(result, ic.Name)
			}
		}
	}
	return result, nil
}

// Get log for a single container of a pod
// Note: callers are responsible for closing the io.ReadCloser
func (k *K8sClient) GetPodLog(podRaw *unstructured.Unstructured, container string) (io.ReadCloser, error) {
	if k.IsValid() {

		pod := &corev1.Pod{}
		if runtime.DefaultUnstructuredConverter == nil {
			return nil, fmt.Errorf("no converter")
		}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(podRaw.Object, &pod)
		if err != nil {
			return nil, err
		}

		clientset, err := kubernetes.NewForConfig(k.config)
		if err != nil {
			return nil, fmt.Errorf("error in getting access to K8S")
		}
		podLogOpts := corev1.PodLogOptions{
			Container: container,
		}
		req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)

		podLogs, err := req.Stream(context.TODO())
		if err != nil {
			return nil, err
		}

		return podLogs, nil

		// buf := new(bytes.Buffer)
		// _, err = io.Copy(buf, podLogs)
		// if err != nil {
		// 	return "error in copy information from podLogs to buf"
		// }
		// str := buf.String()
		// return str

	} else {
		return nil, fmt.Errorf("not connected")
	}
}

func _getK8sClient() *K8sClient {
	return k8sClient
}

func InitLocalK8sClient(configPath *string) {
	logger.Info("Init local k8sclient", zap.String("config", *configPath))
	config, err := clientcmd.BuildConfigFromFlags("", *configPath)
	setupErr := ""

	if err != nil {
		logger.Error("Failed to get client", zap.Error(err))
		config = nil
		setupErr = err.Error()
	}

	k8sClient = &K8sClient{
		config:   config,
		setupErr: setupErr,
	}

	k8sClient.SetupClients()
}

func ToApiVer(userInput string) (string, error) {
	lowerInput := strings.ToLower(userInput)
	allres := GetK8sService().FetchAllApiResources(false)
	if allres != nil {
		for _, resList := range allres.ResList {
			gv := resList.GroupVersion
			for _, res := range resList.APIResources {
				apiVer := gv + "/" + res.Name
				apiVer2 := gv + "/" + res.SingularName
				if lowerInput == res.Name || lowerInput == res.SingularName ||
					lowerInput == apiVer || lowerInput == apiVer2 {
					return apiVer, nil
				}
			}
		}
	}
	return "", fmt.Errorf("no such resource %v", userInput)
}

// userInput could be case insensitive names such as pod, STATEFULSET for built-in types (in cached package)
// or normal all small case names v1/pods, apps/v1/statefusets
func IsTypeSupported(userInput string) (bool, string) {
	if ok, apiVer := common.IsBuiltinTypeSupported(userInput); ok {
		return true, apiVer
	}
	// here userInput must be in the forms of
	// non-builtin simple resource names(including pluralform),
	// or full apiVersion v1/someResource etc.
	if apiVer, err := ToApiVer(userInput); err == nil {
		return true, apiVer
	}
	return false, ""
}

func GetResSpec(res *common.ApiResourceEntry) *common.ResourceSpec {
	spec := &common.ResourceSpec{
		ApiVer: res.ApiVer,
	}

	var err error
	spec.Schema, err = GetK8sService().GetCRDFor(res)
	if err != nil {
		spec.Schema = err.Error()
	}
	return spec
}

func CreateCRFor(res *common.ApiResourceEntry, name string) string {
	if cr, ok := common.SampleCrs[res.ApiVer]; ok {
		return cr
	}
	var builder strings.Builder
	builder.WriteString("apiVersion: " + res.Gv)
	builder.WriteString("\n")
	builder.WriteString("kind: " + res.ApiRes.Kind)
	builder.WriteString("\n")
	builder.WriteString("metadata:\n")
	builder.WriteString("  name: " + name)
	builder.WriteString("\n")

	return builder.String()
}

// Create a instance of apiVer resource
func NewInstance(apiVer string, name string, order int) (*common.ResourceInstance, error) {
	allres := k8sClient.FetchAllApiResources(false)
	if allres != nil {
		if res := allres.FindApiResource(apiVer); res != nil {
			inst := &common.ResourceInstance{
				Spec:     GetResSpec(res),
				InstName: name,
				Cr:       CreateCRFor(res, name),
				Order:    new(int),
			}
			*inst.Order = order
			inst.SetId(uuid.New().String())
			inst.Label = name

			return inst, nil
		}
	}

	//try built in
	if _, ok := common.PossibleUserInputMap[apiVer]; ok {
		inst := common.NewBuiltinInstance(apiVer, order)
		inst.SetName(name)
		inst.SetId(uuid.New().String())
		inst.Label = name

		return inst, nil
	}

	logger.Info("No resource found", zap.String("apiv", apiVer))
	return nil, fmt.Errorf("no such resource %v", apiVer)
}

func (k *K8sClient) FetchGVRInstances(g string, v string, r string, ns string) (*unstructured.UnstructuredList, error) {
	if k.IsValid() {
		gvr := schema.GroupVersionResource{
			Group:    g,
			Version:  v,
			Resource: r,
		}
		instList, err := k.dynClient.Resource(gvr).Namespace(ns).List(context.TODO(), v1.ListOptions{})
		if err != nil {
			return nil, err
		}
		return instList, nil
	}
	return nil, fmt.Errorf("cluster not connected")
}

func (k *K8sClient) FetchAllNamespaces() ([]string, error) {
	allNs := make([]string, 0)

	nsList, err := k.FetchGVRInstances("", "v1", "namespaces", "")
	if err != nil {
		return allNs, err
	}

	for _, ns := range nsList.Items {
		name := ns.GetName()
		allNs = append(allNs, name)
	}
	return allNs, nil
}

func GetPersister() DeploymentPersister {

	cfgDir, err := config.GetConfigDir()
	if err != nil {
		logger.Info("Cannot get config dir", zap.Error(err))
		return &DummyPersister{}
	}
	if !GetK8sService().IsValid() {
		logger.Info("Deployment is disabled because no valid cluster available")
		return &DummyPersister{}
	}

	clustersDir := filepath.Join(cfgDir, "clusters")
	basePath := filepath.Join(clustersDir, GetK8sService().GetClusterName())

	// store cluster index into info.yaml
	infoFile := filepath.Join(clustersDir, "info.yaml")

	clusterInfo := GetK8sService().GetClusterInfo()

	infoMap := make(map[string]*common.ClusterInfo)

	if _, err := os.Stat(infoFile); os.IsNotExist(err) {
		// info.yaml does not exist
		infoMap[clusterInfo.Id] = clusterInfo
		data, err := yamlv3.Marshal(infoMap)
		if err != nil {
			logger.Warn("error writing cluster info", zap.Error(err))
			return &DummyPersister{}
		}
		os.WriteFile(infoFile, data, 0644)
	} else if err != nil {
		logger.Warn("Error checking info.yaml", zap.Error(err))
	} else {
		//exist
		if data, err := os.ReadFile(infoFile); err == nil {
			infos := make(map[string]*common.ClusterInfo, 0)

			if err := yamlv3.Unmarshal(data, infos); err != nil {
				logger.Warn("error unmarshalling cluster info", zap.Error(err))
				return &DummyPersister{}
			}

			infos[clusterInfo.Id] = clusterInfo

			newData, err := yamlv3.Marshal(infos)
			if err != nil {
				logger.Warn("error marshalling cluster info", zap.Error(err))
				return &DummyPersister{}
			}
			os.WriteFile(infoFile, newData, 0644)
		} else {
			logger.Warn("error reading cluster info", zap.Error(err))
		}
	}

	path := filepath.Join(basePath, "deployments")

	if err := os.MkdirAll(path, 0755); err != nil {
		logger.Warn("Cannot get config dir", zap.Error(err))
		return &DummyPersister{}
	}
	fpath := filepath.Join(path, "deployments.yaml")
	persister := &FileDeploymentPersister{
		FilePath: fpath,
		cache:    make([]*DeployDetail, 0),
	}
	return persister
}

type FileDeploymentPersister struct {
	FilePath string
	cache    []*DeployDetail
}

func (fdp *FileDeploymentPersister) persist() error {
	data, err := yamlv3.Marshal(fdp.cache)
	if err != nil {
		return fmt.Errorf("failed to marshal cache to YAML: %w", err)
	}

	err = os.WriteFile(fdp.FilePath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write cache to file: %w", err)
	}

	return nil
}

func (fdp *FileDeploymentPersister) Add(d *DeployDetail) error {
	fdp.cache = append(fdp.cache, d)
	return fdp.persist()
}

// called when some DeployDetail has been changed (like state)
func (fdp *FileDeploymentPersister) Update() error {
	return fdp.persist()
}

func (fdp *FileDeploymentPersister) Remove(d *DeployDetail) error {
	for i, detail := range fdp.cache {
		if detail.Id == d.Id {
			fdp.cache = append(fdp.cache[:i], fdp.cache[i+1:]...)
			break
		}
	}
	return fdp.persist()
}

func (fdp *FileDeploymentPersister) Load() ([]*DeployDetail, error) {

	data, err := os.ReadFile(fdp.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var details []*DeployDetail
	err = yamlv3.Unmarshal(data, &details)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	fdp.cache = details
	return fdp.cache, nil
}

// special comment, don't remove it
//
//go:embed templates/*.tmpl
var rawBuiltinTemplates embed.FS

func registerBuiltinTemplates(gen ktlexplain.Generator) error {

	files, err := rawBuiltinTemplates.ReadDir("templates")
	if err != nil {
		logger.Error("Failed to read files in templates", zap.Error(err))
		return err
	}

	for _, entry := range files {
		contents, err := rawBuiltinTemplates.ReadFile("templates/" + entry.Name())
		if err != nil {
			return err
		}

		err = gen.AddTemplate(
			strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
			string(contents))

		if err != nil {
			return err
		}
	}

	return nil
}

func toRESTMapper(discoveryClient discovery.CachedDiscoveryInterface) (meta.RESTMapper, error) {
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient, func(a string) {
		fmt.Println(a)
	})
	return expander, nil
}

func GetGVR(client discovery.CachedDiscoveryInterface, resEntry *common.ApiResourceEntry) (*schema.GroupVersionResource, []string, error) {
	var fullySpecifiedGVR schema.GroupVersionResource
	var fieldsPath []string
	var err error
	resName := resEntry.ApiRes.Name
	gv := resEntry.Gv //if empty it will be guessed
	mapper, err := toRESTMapper(client)
	if err != nil {
		logger.Error("Failed to get rest mapper", zap.Error(err))
		return nil, nil, err
	}
	if len(gv) == 0 {
		fullySpecifiedGVR, fieldsPath, err = explain.SplitAndParseResourceRequestWithMatchingPrefix(resName, mapper)
		if err != nil {
			return nil, nil, err
		}
	} else {
		fullySpecifiedGVR, fieldsPath, err = explain.SplitAndParseResourceRequest(resName, mapper)
		if err != nil {
			return nil, nil, err
		}
	}

	//outputFormat plaintext
	// Check whether the server reponds to OpenAPIV3.
	if len(gv) > 0 {
		apiVersion, err := schema.ParseGroupVersion(gv)
		if err != nil {
			return nil, nil, err
		}
		fullySpecifiedGVR.Group = apiVersion.Group
		fullySpecifiedGVR.Version = apiVersion.Version
	}
	return &fullySpecifiedGVR, fieldsPath, nil
}

func NewDeployedResources() *DeployedResources {
	dr := &DeployedResources{
		resIds: make(map[string]*DeployDetail),
	}
	dr.persister = GetPersister()
	return dr
}

type DeployedResources struct {
	resIds    map[string]*DeployDetail
	list      []*DeployDetail
	persister DeploymentPersister
}

func (d *DeployedResources) GetPersister() DeploymentPersister {
	return d.persister
}

func (d *DeployedResources) GetSelectedDeployments() []*DeployDetail {
	deps := make([]*DeployDetail, 0)
	for _, itm := range d.list {
		if itm.checkStatus.Value {
			deps = append(deps, itm)
		}
	}
	return deps
}

func (d *DeployedResources) AnySelected() bool {
	for _, itm := range d.list {
		if itm.checkStatus.Value {
			return true
		}
	}
	return false
}

func (d *DeployedResources) Get(index int) *DeployDetail {
	return d.list[index]
}

func (d *DeployedResources) Size() int {
	return len(d.resIds)
}

func (d *DeployedResources) LockAndAdd(resNode common.INode) (map[string]*common.ResourceInstanceAction, error) {
	dd, exists := d.resIds[resNode.GetId()]
	if !exists {
		dd = NewDeployDetail(resNode)
	}

	actions, err := dd.ParseResources()

	// when an empty colleciton is deployed, no actions will be performed
	// and no need to add to the deployedResources
	if len(actions) > 0 && !exists {
		d.AddDetail(dd, true)
	}

	return actions, err
}

func (d *DeployedResources) AddDetail(dd *DeployDetail, persist bool) {
	d.resIds[dd.Id] = dd
	d.list = append(d.list, dd)
	if persist {
		d.persister.Add(dd)
	}
}

// called when deploy failed or undeploy
func (d *DeployedResources) Remove(resId string) {
	delete(d.resIds, resId)
	for i, detail := range d.list {
		if detail.Id == resId {
			d.persister.Remove(detail)
			//d.list = append(d.list[:i], d.list[i+1:]...)
			d.list = slices.Delete(d.list, i, i+1)
			break
		}
	}
}

func (d *DeployedResources) Deployed(resId string, finalNs map[string]types.NamespacedName) {
	d.resIds[resId].Status = common.StateDeployed
	d.resIds[resId].Namespace = common.MapToKeysString(finalNs)
	d.resIds[resId].SetFinalNs(finalNs)
	d.persister.Update()
}

type DeployDetail struct {
	// when loaded from persister
	// if the node cannot be found and restored
	// from the repository's map
	// it means the original resource has been
	// removed by the user, i.e. it became an orphaned
	// deployment
	orphaned bool
	// res may either be a Collection or a ResourceNode
	// in case of collection, the Cr is meaningless
	// it is used to store the collections's desc
	// the Namespace is also ignored. The Kind is
	// "Collection"
	res common.INode
	// need a CR copy to keep the original
	// as the res node may subject to user modification
	// after deployed
	OriginalCrs  map[string]*common.CrInstance             `yaml:"originalcrs,omitempty"`
	AllInstances map[string]*common.ResourceInstanceAction `yaml:"allinstances,omitempty"`
	// The Id is singled out for persistence.
	// We dont persist the whole INode
	Id string `yaml:"id,omitempty"`
	// those fields are for convenience purpose
	Name      string `yaml:"name,omitempty"`
	Namespace string `yaml:"namespace,omitempty"`
	ApiVer    string `yaml:"apiVer,omitempty"`

	Status      common.DeployState `yaml:"status,omitempty"`
	Creation    string             `yaml:"creation,omitempty"`
	checkStatus widget.Bool
	btn         widget.Clickable
}

func (d *DeployDetail) GetAllDeployNamespaces() string {
	var builder strings.Builder
	for _, ns := range d.OriginalCrs {
		builder.WriteString(ns.FinalNs)
		builder.WriteString(" ")
	}
	return strings.TrimSpace(builder.String())
}

func (d *DeployDetail) SetFinalNs(finalNs map[string]types.NamespacedName) {
	for id, ns := range finalNs {
		if inst, ok := d.OriginalCrs[id]; ok {
			inst.FinalNs = ns.Namespace
		}
	}
}

func (d *DeployDetail) SetOrphaned() {
	d.orphaned = true
}

// called after being loaded.
// it tries to restore the res from the repository
func (d *DeployDetail) RestoreNode(n common.INode) {
	d.res = n
}

func (d *DeployDetail) GetClickable() *widget.Clickable {
	return &d.btn
}

func (d *DeployDetail) GetCheckStatus() *widget.Bool {
	return &d.checkStatus
}

func (d *DeployDetail) Merge(newDeploy *DeployDetail) error {
	resActions, err := newDeploy.ParseResources()
	if err != nil {
		return err
	}
	for id, existingAct := range d.AllInstances {
		if newAct, ok := resActions[id]; ok {
			oldCr := d.OriginalCrs[id]
			newCr := newDeploy.OriginalCrs[id]
			if oldCr.Same(newCr) {
				//don't do any action as the resource is not changed
				delete(resActions, id)
			} else {
				newAct.SetAction(common.Update)
			}
		} else {
			//delete the resource
			existingAct.SetAction(common.Delete)
			resActions[id] = existingAct
		}
	}
	//now swap
	d.AllInstances = resActions
	return nil
}

func (d *DeployDetail) ParseResources() (map[string]*common.ResourceInstanceAction, error) {
	var err error = nil
	if len(d.AllInstances) > 0 {
		newDetail := NewDeployDetail(d.res)
		d.Merge(newDetail)
	} else {
		d.AllInstances = make(map[string]*common.ResourceInstanceAction)

		if resNode, ok := d.res.(*common.ResourceNode); ok {
			d.OriginalCrs[d.Id] = common.NewCrInstance(resNode.Instance.GetCR())
			d.ApiVer = resNode.Instance.GetSpecApiVer()
			d.AllInstances[resNode.GetId()] = &common.ResourceInstanceAction{
				Instance:  resNode.Instance,
				Action:    common.Create,
				DefaultNs: resNode.GetDefaultNamespace(),
			}
		} else if col, ok := d.res.(*common.Collection); ok {
			d.ApiVer = common.COLLECTION.ToApiVer()
			allres := col.GetAllResourceInstances()
			for _, r := range allres {
				d.OriginalCrs[r.GetId()] = common.NewCrInstance(r.GetCR())
				d.AllInstances[r.GetId()] = &common.ResourceInstanceAction{
					Instance:  r,
					Action:    common.Create,
					DefaultNs: col.GetDefaultNamespace(),
				}
			}
		} else {
			err = fmt.Errorf("invalid node %v", d.res.GetName())
		}
	}
	return d.AllInstances, err
}

func NewDeployDetail(resNode common.INode) *DeployDetail {

	dd := &DeployDetail{
		res:         resNode,
		Id:          resNode.GetId(),
		OriginalCrs: make(map[string]*common.CrInstance),
		Name:        resNode.GetName(),
		Status:      common.StateNew,
		Creation:    time.Now().Format(time.RFC3339),
	}
	if ownerCol := resNode.GetOwnerCollection(); ownerCol != nil {
		dd.Namespace = ownerCol.GetDefaultNamespace()
	}
	dd.Status = common.StateInDeploy
	return dd
}

type DeploymentPersister interface {
	Add(d *DeployDetail) error
	Update() error
	Remove(d *DeployDetail) error
	Load() ([]*DeployDetail, error)
}

type DummyPersister struct {
}

// Load implements DeploymentPersister.
func (d *DummyPersister) Load() ([]*DeployDetail, error) {
	return nil, nil
}

// Remove implements DeploymentPersister.
func (*DummyPersister) Remove(d *DeployDetail) error {
	return nil
}

// Save implements DeploymentPersister.
func (*DummyPersister) Add(d *DeployDetail) error {
	return nil
}

// Update implements DeploymentPersister.
func (*DummyPersister) Update() error {
	return nil
}
