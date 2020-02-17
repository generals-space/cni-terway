package netconf

import (
	"github.com/containernetworking/cni/pkg/types"
)

// NetConf `/etc/cni/net.d/`目录下配置文件对象.
type NetConf struct {
	types.NetConf
	ServiceIPCIDR string `json:"serviceIPCIDR"`
	Delegate      map[string]interface{}
}
