package k8sservice

import (
	"context"
	"encoding/json"
	"net"

	"gaohoward.tools/k8s/resutil/pkg/common"
	"go.uber.org/zap"
	grpc "google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type server struct {
	GrpcK8SServiceServer
	client *K8sClient
}

func (s *server) IsValid(ctx context.Context, req *emptypb.Empty) (*wrapperspb.BoolValue, error) {
	result := wrapperspb.BoolValue{
		Value: s.client.IsValid(),
	}
	return &result, nil
}

func (s *server) DeployResource(_ context.Context, resReq *DeployResourceRequest) (*DeployResourceReply, error) {
	res := NewResourceInstanceAction(resReq)
	nsn, err := s.client.DeployResource(res, resReq.TargetNs)
	if err != nil {
		return nil, err
	}

	reply := &DeployResourceReply{
		Name:      nsn.Name,
		Namespace: nsn.Namespace,
	}

	return reply, nil
}

func (s *server) GetClusterInfo(context.Context, *emptypb.Empty) (*ClusterInfoReply, error) {
	clusterInfo := s.client.GetClusterInfo()
	reply := ClusterInfoReply{
		Host: clusterInfo.Host,
		Id:   clusterInfo.Id,
	}
	return &reply, nil
}

func (s *server) FetchAllApiResources(ctx context.Context, req *wrapperspb.BoolValue) (*ApiResourceInfoReply, error) {
	allRes := s.client.FetchAllApiResources(req.Value)

	resList := make([]string, 0)
	for _, l := range allRes.ResList {
		if ljson, err := json.Marshal(l); err == nil {
			resList = append(resList, string(ljson))
		} else {
			logger.Warn("failed to marshal res list", zap.Any("list", l))
		}
	}

	resMap := make(map[string]string)
	for k, v := range allRes.ResMap {
		if vjson, err := json.Marshal(v); err == nil {
			resMap[k] = string(vjson)
		} else {
			logger.Warn("failed to marshal res map", zap.Any("item", v))
		}
	}

	reply := ApiResourceInfoReply{
		Cached:              allRes.Cached,
		ApiResourceListJson: resList,
		ResMap:              resMap,
	}
	return &reply, nil
}

func (s *server) FetchGVRInstances(ctx context.Context, req *FetchGvrRequest) (*GvrReply, error) {
	result, err := s.client.FetchGVRInstances(req.G, req.V, req.R, req.Ns)
	if err != nil {
		return nil, err
	}

	rjson, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	return &GvrReply{
		UnstructuredListJson: string(rjson),
	}, nil
}

func (s *server) FetchAllNamespaces(context.Context, *emptypb.Empty) (*AllNamespacesReply, error) {
	allNs, err := s.client.FetchAllNamespaces()
	if err != nil {
		return nil, err
	}
	return &AllNamespacesReply{
		Namespaces: allNs,
	}, nil
}

func (s *server) GetPodLog(req *PodLogRequest, streamServer grpc.ServerStreamingServer[wrapperspb.StringValue]) error {

	podRaw := &unstructured.Unstructured{}

	err := json.Unmarshal([]byte(req.PodRawJson), podRaw)
	if err != nil {
		return err
	}

	logReader, err := s.client.GetPodLog(podRaw, req.Container)

	if err != nil {
		logger.Info("error getting pod log", zap.Error(err))
		return err
	}
	defer logReader.Close()

	for {
		bts := make([]byte, 2048)
		n, err := logReader.Read(bts)

		if n > 0 {
			msg := wrapperspb.StringValue{
				Value: string(bts[:n]),
			}
			streamServer.Send(&msg)
		} else if err != nil {
			// here we just return nil
			// grpc will take care and return correct EOF
			return nil
		}
	}
}

func (s *server) GetPodContainers(ctx context.Context, req *wrapperspb.StringValue) (*GetPodContainersReply, error) {

	podRaw := &unstructured.Unstructured{}

	err := json.Unmarshal([]byte(req.Value), podRaw)
	if err != nil {
		return nil, err
	}

	containers, err := s.client.GetPodContainers(podRaw)
	if err != nil {
		return nil, err
	}

	return &GetPodContainersReply{
		Containers: containers,
	}, nil
}

func (s *server) GetClusterName(context.Context, *emptypb.Empty) (*wrapperspb.StringValue, error) {
	return &wrapperspb.StringValue{
		Value: s.client.GetClusterName(),
	}, nil
}

func (s *server) GetCRDFor(ctx context.Context, req *ApiResourceEntry) (*CrdReply, error) {

	entry := &common.ApiResourceEntry{
		ApiVer: req.ApiVer,
		Gv:     req.Gv,
		Schema: req.Schema,
	}
	apiRes := &v1.APIResource{}
	err := json.Unmarshal([]byte(req.ApiResourceJson), apiRes)
	if err != nil {
		return nil, err
	}
	entry.ApiRes = apiRes

	crd, err := s.client.GetCRDFor(entry)
	if err != nil {
		return nil, err
	}

	return &CrdReply{
		Crd: crd,
	}, nil
}

func NewResourceInstanceAction(req *DeployResourceRequest) *common.ResourceInstanceAction {
	action := common.ResourceInstanceAction{}
	action.Action = common.ResourceAction(req.Action)
	action.DefaultNs = req.DefaultNs
	action.Instance = &common.ResourceInstance{
		Id: req.Id,
		Spec: &common.ResourceSpec{
			ApiVer: req.Spec.ApiVer,
			Schema: req.Spec.Schema,
			Loaded: &req.Spec.Loaded,
		},
		Cr:       req.Cr,
		Order:    int32PtrToIntPtr(&req.Order),
		InstName: req.InstName,
		Label:    req.Label,
	}
	return &action
}

// int32PtrToIntPtr converts a *int32 to a *int.
func int32PtrToIntPtr(i *int32) *int {
	if i == nil {
		return nil
	}
	v := int(*i)
	return &v
}

func Run() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer()

	RegisterGrpcK8SServiceServer(s, &server{
		client: _getK8sClient(),
	})

	logger.Info("server listening", zap.String("port", "8080"))
	if err := s.Serve(listener); err != nil {
		logger.Error("failed to serve", zap.Error(err))
	}
}
