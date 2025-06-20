package common

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"gaohoward.tools/k8s/resutil/pkg/logs"
	"github.com/google/uuid"
	"go.uber.org/zap"
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
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
)

var k8sClient *K8sClient

var NoApiResourceInfo = ApiResourceInfo{}

type K8sClient struct {
	lock            sync.RWMutex
	config          *rest.Config
	discoveryClient *discovery.DiscoveryClient
	mapper          *restmapper.DeferredDiscoveryRESTMapper
	dynClient       *dynamic.DynamicClient
	setupErr        string
	allRes          *ApiResourceInfo
	resUtil         *ResUtil
	clusterInfo     *ClusterInfo
}

func (k *K8sClient) GetClusterName() string {
	return k.clusterInfo.Id
}

func (k *K8sClient) GetClusterInfo() *ClusterInfo {
	return k.clusterInfo
}

func (k *K8sClient) FetchAllApiResources(force bool) *ApiResourceInfo {
	k.lock.Lock()
	defer k.lock.Unlock()

	fetched := false

	if force && k.IsValid() {
		k.allRes = &ApiResourceInfo{
			ResList: make([]*v1.APIResourceList, 0),
			ResMap:  make(map[string]*ApiResourceEntry, 0),
		}
		apiResourceList, err := k.discoveryClient.ServerPreferredResources()
		if err != nil {
			logger.Error("Error fetching API resources", zap.Error(err))
		} else {
			fetched = true
			if len(k.allRes.ResList) > 0 {
				k.allRes.ResList = make([]*v1.APIResourceList, 0)
			}
			k.allRes.ResList = append(k.allRes.ResList, apiResourceList...)
			//fill the map
			if len(k.allRes.ResList) > 0 {
				for _, resList := range k.allRes.ResList {
					for _, res := range resList.APIResources {
						key := resList.GroupVersion + "/" + res.Name
						entry := &ApiResourceEntry{
							ApiVer: key,
							Gv:     resList.GroupVersion,
							ApiRes: &res,
						}
						if sch, err := k.resUtil.GetCRDFor(entry); err == nil {
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

	if !fetched {
		var err error
		k.allRes, err = GetCachedApiResourceList()
		if err != nil {
			logger.Info("failed to fetch cached api-resources", zap.String("error", err.Error()))
		} else {
			k.allRes.Cached = true
		}
	} else {
		k.allRes.Cached = false
	}

	if k.allRes != nil && !k.allRes.Cached {
		persister := GetApiResourcePersister()
		persister.Save(k.allRes)
		logger.Info("saved updated api-resources")
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

func (k *K8sClient) DeployResource(res *ResourceInstanceAction, targetNs string) (types.NamespacedName, error) {

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
	case Create:
		appLogger.Info("CREATE resource", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()))
		if _, err := dr.Apply(context.TODO(), obj.GetName(), obj, v1.ApplyOptions{FieldManager: APP_NAME}); err != nil {
			appLogger.Error("Failed to create resource", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()))
			logger.Error("Failed to Create resource", zap.Error(err))
			return finalNamespace, err
		}
		appLogger.Info("Resource created successfully", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()))
	case Delete:
		appLogger.Info("DELETE resource", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()))
		if err := dr.Delete(context.TODO(), obj.GetName(), v1.DeleteOptions{}); err != nil {
			appLogger.Error("Failed to delete resource", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()), zap.Error(err))
			logger.Error("Failed to delete resource", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()), zap.Error(err))
			return finalNamespace, err
		}
		appLogger.Info("Resource deleted successfully", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()))
	case Update:
		appLogger.Info("UPDATE resource", zap.String("name", obj.GetName()), zap.String("ns", obj.GetNamespace()))
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			// ApplyPatchType means server side apply
			if _, err := dr.Patch(context.TODO(), obj.GetName(), types.ApplyPatchType, data, v1.PatchOptions{FieldManager: APP_NAME}); err != nil {
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

		k.clusterInfo = &ClusterInfo{
			Host: k.config.Host,
			Id:   clientId,
		}
	} else {
		k.clusterInfo = &ClusterInfo{}
		k.setupErr = "No rest config"
	}
	k.resUtil = createResUtil(k.config)
}

func (k *K8sClient) GetResUtil() *ResUtil {
	return k.resUtil
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

func GetK8sClient() *K8sClient {
	return k8sClient
}

func InitK8sClient(configPath *string) {
	config, err := clientcmd.BuildConfigFromFlags("", *configPath)

	if err != nil {
		logger.Error("Failed to get client", zap.Error(err))
		config = nil
	}
	k8sClient = &K8sClient{
		config:   config,
		setupErr: "",
	}

	k8sClient.SetupClients()

}

func ToApiVer(userInput string) (string, error) {
	lowerInput := strings.ToLower(userInput)
	allres := k8sClient.FetchAllApiResources(false)
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
	return "", fmt.Errorf("no such resource %v", userInput)
}

// userInput could be case insensitive names such as pod, STATEFULSET for built-in types (in cached package)
// or normal all small case names v1/pods, apps/v1/statefusets
func IsTypeSupported(userInput string) (bool, string) {
	if ok, apiVer := IsBuiltinTypeSupported(userInput); ok {
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

func GetResSpec(res *ApiResourceEntry) *ResourceSpec {
	spec := &ResourceSpec{
		ApiVer: res.ApiVer,
	}
	resUtil := GetK8sClient().GetResUtil()
	var err error
	spec.Schema, err = resUtil.GetCRDFor(res)
	if err != nil {
		spec.Schema = err.Error()
	}
	return spec
}

func CreateCRFor(res *ApiResourceEntry, name string) string {
	if cr, ok := SampleCrs[res.ApiVer]; ok {
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
func NewInstance(apiVer string, name string, order int) (*ResourceInstance, error) {
	allres := k8sClient.FetchAllApiResources(false)
	if allres != nil {
		if res := allres.FindApiResource(apiVer); res != nil {
			inst := &ResourceInstance{
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
	if _, ok := PossibleUserInputMap[apiVer]; ok {
		inst := NewBuiltinInstance(apiVer, order)
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
