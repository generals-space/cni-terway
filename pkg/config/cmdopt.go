package config

import (
	"github.com/vishvananda/netlink"
	"k8s.io/klog"

	"github.com/generals-space/cni-terway/pkg/cninet"
)

// CmdOpts 命令行参数对象
type CmdOpts struct {
	BridgeName string
	// 集群之间通信所使用的主网卡.
	// 如果不是多网卡环境, 一般为 eth0
	Eth0Name string
}

// Complete 使用默认值补全 CmdOpts 中未指定的选项
func (opt *CmdOpts) Complete() (err error) {
	// 如果未显式指定目标网络接口, 则尝试通过宿主机的默认路由获取其绑定的接口.
	if opt.Eth0Name == "" {
		klog.Info("doesn't specify main network interface, try to find it")
		r, err := cninet.GetDefRoute()
		if err != nil {
			return err
		}
		link, err := netlink.LinkByIndex(r.LinkIndex)
		if err != nil {
			return err
		}
		opt.Eth0Name = link.Attrs().Name
	}
	return
}
