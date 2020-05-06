package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/generals-space/cni-terway/pkg/serviceipcidr"
	"k8s.io/klog"
)

// NetConf `/etc/cni/net.d/`目录下配置文件对象.
type NetConf struct {
	types.NetConf
	ServiceIPCIDR string `json:"serviceIPCIDR"`
	Delegate      map[string]interface{}
	// ServerSocket cni server的socket路径.
	// cni server是用来设置容器内部为固定IP的.
	ServerSocket string `json:"server_socket"`
}

// Complete 从 apiserver 获取 service cidr 范围, 然后写入到 cni netconf 文件中.
func (netConf *NetConf) Complete(netConfPath string) (err error) {
	netConfContent, err := ioutil.ReadFile(netConfPath)
	if err != nil {
		return fmt.Errorf("failed to read cni netconf file: %s", err)
	}

	err = json.Unmarshal(netConfContent, netConf)
	if err != nil {
		return fmt.Errorf("failed to unmarshal cni netconf content: %s", err)
	}

	serviceIPCIDR, err := serviceipcidr.GetServiceIPCIDR()
	if err != nil {
		return fmt.Errorf("failed to get service ip cidr: %s", err)
	}
	klog.Infof("get service ip cidr: %s", serviceIPCIDR)
	netConf.ServiceIPCIDR = serviceIPCIDR

	netConfContent, err = json.Marshal(netConf)
	if err != nil {
		return fmt.Errorf("failed to marshal cni netconf: %s", err)
	}

	err = ioutil.WriteFile(netConfPath, netConfContent, 0644)
	if err != nil {
		return fmt.Errorf("failed to write into cni netconf: %s", err)
	}

	return
}
