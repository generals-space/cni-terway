package netconf

import (
	"github.com/containernetworking/cni/pkg/types"
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
