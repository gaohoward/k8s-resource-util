package k8sservice

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gaohoward.tools/k8s/resutil/pkg/common"
	"gaohoward.tools/k8s/resutil/pkg/logs"
	"gaohoward.tools/k8s/resutil/pkg/options"
	"go.uber.org/zap"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

var logger *zap.Logger

func init() {
	logger, _ = logs.NewAppLogger("k8sservice")
}

type K8sService interface {
	IsValid() bool
	DeployResource(res *common.ResourceInstanceAction, targetNs string) (types.NamespacedName, error)
	GetClusterInfo() *common.ClusterInfo
	FetchAllApiResources(force bool) *common.ApiResourceInfo
	FetchGVRInstances(g string, v string, r string, ns string) (*unstructured.UnstructuredList, error)
	FetchAllNamespaces() ([]string, error)
	GetPodLog(podRaw *unstructured.Unstructured, container string) (io.ReadCloser, error)
	GetPodContainers(podRaw *unstructured.Unstructured) ([]string, error)
	GetClusterName() string
	GetCRDFor(resEntry *common.ApiResourceEntry) (string, error)
}

type LocalK8sService struct {
	localClient *K8sClient
}

// GetCRDFor implements K8sService.
func (l *LocalK8sService) GetCRDFor(resEntry *common.ApiResourceEntry) (string, error) {
	return l.localClient.GetCRDFor(resEntry)
}

// DeployResource implements K8sService.
func (l *LocalK8sService) DeployResource(res *common.ResourceInstanceAction, targetNs string) (types.NamespacedName, error) {
	return l.localClient.DeployResource(res, targetNs)
}

// FetchAllApiResources implements K8sService.
func (l *LocalK8sService) FetchAllApiResources(force bool) *common.ApiResourceInfo {
	apiInfo := l.localClient.FetchAllApiResources(force)

	if apiInfo != nil && !apiInfo.Cached {
		persister := common.GetApiResourcePersister()
		persister.Save(apiInfo)
		logger.Info("saved updated api-resources")
	}

	return apiInfo
}

// FetchAllNamespaces implements K8sService.
func (l *LocalK8sService) FetchAllNamespaces() ([]string, error) {
	return l.localClient.FetchAllNamespaces()
}

// FetchGVRInstances implements K8sService.
func (l *LocalK8sService) FetchGVRInstances(g string, v string, r string, ns string) (*unstructured.UnstructuredList, error) {
	return l.localClient.FetchGVRInstances(g, v, r, ns)
}

// GetClusterInfo implements K8sService.
func (l *LocalK8sService) GetClusterInfo() *common.ClusterInfo {
	return l.localClient.GetClusterInfo()
}

// GetClusterName implements K8sService.
func (l *LocalK8sService) GetClusterName() string {
	return l.localClient.GetClusterName()
}

// GetPodContainers implements K8sService.
func (l *LocalK8sService) GetPodContainers(podRaw *unstructured.Unstructured) ([]string, error) {
	return l.localClient.GetPodContainers(podRaw)
}

// GetPodLog implements K8sService.
func (l *LocalK8sService) GetPodLog(podRaw *unstructured.Unstructured, container string) (io.ReadCloser, error) {
	return l.localClient.GetPodLog(podRaw, container)
}

// IsValid implements K8sService.
func (l *LocalK8sService) IsValid() bool {
	return l.localClient.IsValid()
}

type RemoteK8sService struct {
	Conn *grpc.ClientConn
}

// GetCRDFor implements K8sService.
func (r *RemoteK8sService) GetCRDFor(resEntry *common.ApiResourceEntry) (string, error) {
	if r.Conn == nil {
		return "", fmt.Errorf("no connection")
	}

	grpcClient := NewGrpcK8SServiceClient(r.Conn)

	entry := ApiResourceEntry{
		ApiVer: resEntry.ApiVer,
		Gv:     resEntry.Gv,
		Schema: resEntry.Schema,
	}

	resJson, err := json.Marshal(resEntry.ApiRes)
	if err != nil {
		logger.Error("failed to marsh api res", zap.Error(err))
		return "", err
	}

	entry.ApiResourceJson = string(resJson)

	reply, err := grpcClient.GetCRDFor(context.Background(), &entry)

	if err != nil {
		logger.Error("failed rpc call", zap.Error(err))
		return "", err
	}

	if reply.Error != "" {
		return "", fmt.Errorf(reply.Error)
	}

	return reply.Crd, nil
}

// DeployResource implements K8sService.
func (r *RemoteK8sService) DeployResource(res *common.ResourceInstanceAction, targetNs string) (types.NamespacedName, error) {
	if r.Conn == nil {
		return types.NamespacedName{}, fmt.Errorf("no connection")
	}

	grpcClient := NewGrpcK8SServiceClient(r.Conn)

	request := DeployResourceRequest{}
	request.Action = int32(res.Action)
	request.Cr = res.Instance.Cr
	request.DefaultNs = res.DefaultNs
	request.Id = res.Instance.Id
	request.Order = int32(*res.Instance.Order)
	request.Spec = &ResourceSpec{
		ApiVer: res.Instance.Spec.ApiVer,
		Schema: res.Instance.Spec.Schema,
	}
	if res.Instance.Spec.Loaded != nil {
		request.Spec.Loaded = *res.Instance.Spec.Loaded
	}
	request.InstName = res.Instance.InstName
	request.Label = res.Instance.Label
	request.TargetNs = targetNs

	reply, err := grpcClient.DeployResource(context.Background(), &request)
	if err != nil {
		return types.NamespacedName{}, err
	}
	return types.NamespacedName{Name: reply.Name, Namespace: reply.Namespace}, nil
}

// FetchAllApiResources implements K8sService.
func (r *RemoteK8sService) FetchAllApiResources(force bool) *common.ApiResourceInfo {
	if r.Conn == nil {
		return nil
	}

	grpcClient := NewGrpcK8SServiceClient(r.Conn)

	request := &wrapperspb.BoolValue{
		Value: force,
	}
	reply, err := grpcClient.FetchAllApiResources(context.Background(), request)
	if err != nil {
		logger.Error("failed rpc call", zap.Error(err))
		return nil
	}

	apiResList := make([]*v1.APIResourceList, 0)

	for _, apiList := range reply.ApiResourceListJson {
		aList := v1.APIResourceList{}
		err := json.Unmarshal([]byte(apiList), &aList)
		if err != nil {
			logger.Warn("failed to unmarshal list", zap.Error(err))
		} else {
			apiResList = append(apiResList, &aList)
		}
	}

	resMap := make(map[string]*common.ApiResourceEntry)

	for k, v := range reply.ResMap {
		entry := common.ApiResourceEntry{}
		err := json.Unmarshal([]byte(v), &entry)
		if err != nil {
			logger.Warn("failed to unmarshal entry", zap.Error(err))
		} else {
			resMap[k] = &entry
		}
	}

	apiInfo := &common.ApiResourceInfo{
		Cached:  reply.Cached,
		ResList: apiResList,
		ResMap:  resMap,
	}

	if !apiInfo.Cached {
		persister := common.GetApiResourcePersister()
		persister.Save(apiInfo)
		logger.Info("saved updated api-resources")
	}

	return apiInfo
}

// FetchAllNamespaces implements K8sService.
func (r *RemoteK8sService) FetchAllNamespaces() ([]string, error) {
	if r.Conn == nil {
		return nil, fmt.Errorf("no connection")
	}

	grpcClient := NewGrpcK8SServiceClient(r.Conn)
	empty := emptypb.Empty{}
	reply, err := grpcClient.FetchAllNamespaces(context.Background(), &empty)
	if err != nil {
		return nil, fmt.Errorf("failed rpc call %v", err)
	}
	if reply.Error != "" {
		return nil, fmt.Errorf("remote service error: %s", reply.Error)
	}
	return reply.Namespaces, nil
}

// FetchGVRInstances implements K8sService.
func (r *RemoteK8sService) FetchGVRInstances(g string, v string, res string, ns string) (*unstructured.UnstructuredList, error) {
	if r.Conn == nil {
		return nil, fmt.Errorf("no connection")
	}

	grpcClient := NewGrpcK8SServiceClient(r.Conn)

	request := &FetchGvrRequest{
		G:  g,
		V:  v,
		R:  res,
		Ns: ns,
	}
	reply, err := grpcClient.FetchGVRInstances(context.Background(), request)
	if err != nil {
		return nil, fmt.Errorf("failed rpc call %v", err)
	}

	result := &unstructured.UnstructuredList{}
	if reply.UnstructuredListJson != "" {
		err := json.Unmarshal([]byte(reply.UnstructuredListJson), result)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal unstructured list: %w", err)
		}
		return result, nil
	}
	return nil, fmt.Errorf("no unstructured list returned from remote service")
}

// GetClusterInfo implements K8sService.
func (r *RemoteK8sService) GetClusterInfo() *common.ClusterInfo {
	if r.Conn == nil {
		return nil
	}

	grpcClient := NewGrpcK8SServiceClient(r.Conn)

	empty := emptypb.Empty{}

	reply, err := grpcClient.GetClusterInfo(context.Background(), &empty)

	if err != nil {
		return nil
	}

	clusterInfo := &common.ClusterInfo{
		Host: reply.Host,
		Id:   reply.Id,
	}

	return clusterInfo
}

// GetClusterName implements K8sService.
func (r *RemoteK8sService) GetClusterName() string {
	if r.Conn == nil {
		return ""
	}

	grpcClient := NewGrpcK8SServiceClient(r.Conn)

	empty := emptypb.Empty{}

	value, err := grpcClient.GetClusterName(context.Background(), &empty)

	if err != nil {
		return ""
	}
	return value.Value
}

// GetPodContainers implements K8sService.
func (r *RemoteK8sService) GetPodContainers(podRaw *unstructured.Unstructured) ([]string, error) {
	if r.Conn == nil {
		return nil, fmt.Errorf("no remote connection")
	}

	grpcClient := NewGrpcK8SServiceClient(r.Conn)

	podJson, err := json.Marshal(podRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pod: %w", err)
	}

	podStr := wrapperspb.StringValue{
		Value: string(podJson),
	}

	reply, err := grpcClient.GetPodContainers(context.Background(), &podStr)

	if err != nil {
		return nil, fmt.Errorf("failed rpc call %v", err)
	}

	return reply.GetContainers(), nil
}

type LogReader struct {
	streamClient grpc.ServerStreamingClient[wrapperspb.StringValue]
}

// Close implements io.ReadCloser.
func (l *LogReader) Close() error {
	return nil
}

// Read implements io.ReadCloser.
func (l *LogReader) Read(p []byte) (n int, err error) {
	logSeg, err := l.streamClient.Recv()

	num := 0
	if logSeg != nil {
		copy(p, []byte(logSeg.Value))
		num = len(logSeg.Value)
	}

	if err != nil {
		return num, err
	}

	return num, nil
}

func NewLogReader(streamClient grpc.ServerStreamingClient[wrapperspb.StringValue]) (io.ReadCloser, error) {
	return &LogReader{
		streamClient: streamClient,
	}, nil
}

// GetPodLog implements K8sService.
func (r *RemoteK8sService) GetPodLog(podRaw *unstructured.Unstructured, container string) (io.ReadCloser, error) {
	if r.Conn == nil {
		return nil, fmt.Errorf("no remote connection")
	}

	grpcClient := NewGrpcK8SServiceClient(r.Conn)

	request := &PodLogRequest{
		Container: container,
	}
	podBytes, err := json.Marshal(podRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pod: %w", err)
	}
	request.PodRawJson = string(podBytes)

	streamClient, err := grpcClient.GetPodLog(context.Background(), request)

	if err != nil {
		return nil, fmt.Errorf("failed rpc call %v", err)
	}

	// wrap it in io.readcloser
	return NewLogReader(streamClient)
}

// IsValid implements K8sService.
func (r *RemoteK8sService) IsValid() bool {
	if r.Conn == nil {
		return false
	}

	grpcClient := NewGrpcK8SServiceClient(r.Conn)

	result, err := grpcClient.IsValid(context.Background(), &emptypb.Empty{})

	if err != nil {
		logger.Error("failed rpc", zap.Error(err))
		return false
	}
	return result.Value
}

var k8sService K8sService

func NewRemoteK8sService(agentUrl string) *RemoteK8sService {

	service := &RemoteK8sService{}

	parts := strings.Split(agentUrl, ":")
	host := parts[0]
	var port string
	if len(parts) == 2 {
		port = parts[1]
	} else {
		port = "8080"
	}
	url := host + ":" + port

	opts := grpc.WithTransportCredentials(insecure.NewCredentials())

	cc, err := grpc.NewClient(url, opts)
	if err != nil {
		logger.Error("failed to init remote client", zap.Error(err))
		service.Conn = nil
	} else {
		logger.Info("k8s service connection", zap.String("url", url))
		service.Conn = cc
	}

	return service
}

func NewLocalK8sService() *LocalK8sService {
	return &LocalK8sService{
		localClient: _getK8sClient(),
	}
}

func InitK8sService() {

	// kubeconfig can be like agent=host:7555
	if strings.HasPrefix(options.Options.Kubeconfig, "agent=") {

		agentUrl := strings.TrimPrefix(options.Options.Kubeconfig, "agent=")

		k8sService = NewRemoteK8sService(agentUrl)
	} else {
		InitLocalK8sClient(&options.Options.Kubeconfig)
		k8sService = NewLocalK8sService()
	}
}

func GetK8sService() K8sService {
	return k8sService
}
