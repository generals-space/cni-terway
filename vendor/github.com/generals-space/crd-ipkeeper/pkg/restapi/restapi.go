package restapi

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/parnurzeal/gorequest"
)

// PodRequest 由CNI插件调用时传入的pause容器信息
type PodRequest struct {
	PodName      string `json:"pod_name"`
	PodNamespace string `json:"pod_namespace"`
	ContainerID  string `json:"container_id"`
	NetNs        string `json:"net_ns"`
	// cni插件使用的网桥设备的名称, 一般默认为cni0.
	CNI0 string `json:"cni0"`
}

// PodResponse ...
type PodResponse struct {
	// IPAddress 点分十进制+掩码字符串, 如`192.168.0.1/24`
	IPAddress string `json:"address"`
	Gateway    string `json:"gateway"`
	DoNothing bool   `json:"do_nothing"`
}

// CNIServerClient ...
type CNIServerClient struct {
	*gorequest.SuperAgent
}

// NewCNIServerClient 由CNI插件调用以进行初始化,
// 之后可以调用该client对象的Add/Del方法.
func NewCNIServerClient(socketAddress string) *CNIServerClient {
	request := gorequest.New()
	request.Transport = &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", socketAddress)
		},
	}
	return &CNIServerClient{request}
}

// Add CNI插件在pause插件创建完成后, 准备部署网络时调用此方法.
// @param podReq: 由CNI插件调用时传入的pause容器信息.
func (csc *CNIServerClient) Add(podReq *PodRequest) (*PodResponse, error) {
	resp := &PodResponse{}
	// 貌似与unix socket建立的http连接, 就是通过这种URL(dummy)进行访问的?
	res, body, errors := csc.Post("http://dummy/api/v1/add").Send(podReq).EndStruct(resp)
	if len(errors) != 0 {
		return nil, errors[0]
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("request ip return %d %s", res.StatusCode, body)
	}
	return resp, nil
}

// Del ...
func (csc *CNIServerClient) Del(podReq *PodRequest) error {
	// 貌似与unix socket建立的http连接, 就是通过这种URL(dummy)进行访问的?
	res, body, errors := csc.Post("http://dummy/api/v1/del").Send(podReq).End()
	if len(errors) != 0 {
		return errors[0]
	}
	if res.StatusCode != 204 {
		return fmt.Errorf("delete ip return %d %s", res.StatusCode, body)
	}
	return nil
}
