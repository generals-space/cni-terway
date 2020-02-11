package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/vishvananda/netlink"
	"k8s.io/klog"
)

const (
	bridgeName = "mybr0"
	eth0Name   = "ens160"
)

var versionAll = version.PluginSupports("0.3.1")

// NetConf `/etc/cni/net.d/`目录下配置文件对象.
type NetConf struct {
	types.NetConf
	Delegate map[string]interface{} `json:"delegate"`
}

func startDHCP(ctx context.Context) (err error) {
	/*
		if util.Exists("/run/cni/dhcp.sock") {
			klog.Info("dhcp.sock already exist")
			return
		}
		klog.Info("dhcp.sock doesn't exist, continue.")
	*/
	err = os.Remove("/run/cni/dhcp.sock")
	if err != nil {
		if err.Error() != "remove /run/cni/dhcp.sock: no such file or directory" {
			klog.Errorf("try to rm dhcp.sock failed: %s", err)
			return
		}
		// 目标不存在, 则继续.
	}

	c := exec.CommandContext(ctx, "/opt/cni/bin/dhcp", "daemon")
	stdout := &bytes.Buffer{}
	// c.Env =
	// c.Stdin =
	c.Stdout = stdout
	c.Stderr = stdout
	if err := c.Run(); err != nil {
		klog.Errorf("dhcp start failed: %s, stdout: %s, stderr: %s", err, c.Stdout, c.Stderr)
		return err
	}
	return nil
}

// 手动创建 mybr0 接口, 然后将eth0接入, 因为如果不完成接入,
// invoke调用bridge+dhcp插件时请求会失败, 出现如下报错
// error calling DHCP.Allocate: no more tries
func linkMasterBridge() (err error) {
	linkBridge, err := netlink.LinkByName(bridgeName)
	if err != nil {
		if err.Error() != "Link not found" {
			klog.Errorf("failed to get bridge %s: %s.", bridgeName, err)
			return
		}
		klog.Warningf("bridge: %s doesn't exist, try to create it manually.", bridgeName)
		// 尝试手动创建然后启动
		bridge := &netlink.Bridge{
			LinkAttrs: netlink.LinkAttrs{
				Name: bridgeName,
			},
		}
		err = netlink.LinkAdd(bridge)
		if err != nil {
			klog.Warningf("failed to craete bridge %s: %s", bridgeName, err)
			return
		}
		err = netlink.LinkSetUp(bridge)
		if err != nil {
			klog.Warningf("failed to setup bridge %s: %s", bridgeName, err)
			return
		}
		linkBridge, err = netlink.LinkByName(bridgeName)
		if err != nil {
			klog.Warningf("failed to get bridge %s: %s", bridgeName, err)
			return
		}
	}

	linkEth0, err := netlink.LinkByName(eth0Name)
	if err != nil {
		klog.Errorf("failed to get target device %s: %s", eth0Name, err)
		return err
	}
	addrs, err := netlink.AddrList(linkEth0, netlink.FAMILY_V4)
	if err != nil {
		klog.Errorf("failed to get address list on %s: %s", eth0Name, err)
		return
	}
	klog.V(3).Infof("get addresses of device %s, len: %d: %+v", eth0Name, len(addrs), addrs)

	// 之后要由bridge设备接管eth0设备的所有路由
	routes, err := netlink.RouteList(linkEth0, netlink.FAMILY_V4)
	if err != nil {
		klog.Errorf("failed to get address list on %s: %s", eth0Name, err)
		return
	}
	klog.V(3).Infof("get route items of device %s, len: %d: %+v", eth0Name, len(routes), routes)

	err = netlink.LinkSetMaster(linkEth0, linkBridge)
	if err != nil {
		return err
	}
	klog.V(3).Infof("set %s master %s success", eth0Name, bridgeName)

	for _, addr := range addrs {
		// 从一个接口上移除IP地址, 对应的路由应该也会被移除.
		err = netlink.AddrDel(linkEth0, &addr)
		if err != nil {
			klog.Errorf("failed to del addr on %s: %s", eth0Name, err)
			continue
		}
		// 不知道Label是什么意思, 但是如果不修改这个字段, 会出现如下问题.
		// label must begin with interface name
		// 在AddrAdd()源码中有检验Label是否以接口名称为前缀的判断.
		addr.Label = bridgeName
		err = netlink.AddrAdd(linkBridge, &addr)
		if err != nil {
			klog.Errorf("failed to add addr to %s: %s", bridgeName, err)
			continue
		}
	}
	klog.V(3).Infof("move address from %s to %s success", eth0Name, bridgeName)

	/*
		// 修改原网卡的IP地址为0.0.0.0(没有掩码), 不过在运行到这里的时候会panic, 应该怎么做还没想好. ???
		newAddr := &netlink.Addr{
			IPNet: &net.IPNet{IP: net.ParseIP("0.0.0.0")},
		}
		err = netlink.AddrAdd(linkEth0, newAddr)
		if err != nil {
			return
		}
	*/

	// 修改路由, 这里是逆向遍历, 因为routes[0]为默认路由, 在目标路由不指定的情况下添加default会失败.
	// Error: Nexthop has invalid gateway.
	// 所以我们先修改指定目标网络的路由, 如
	// 192.168.0.0/24 dev eth0 proto kernel scope link src 192.168.0.105 metric 100
	length := len(routes)
	for i := 0; i < length; i++ {
		route := routes[length-i-1]
		err = netlink.RouteDel(&route)
		if err != nil {
			// 有可能在移除eth0上的IP时, 对应的路由就自动被移除了, 所以这里出错不return
			klog.Errorf("failed to del route %+v from: %s", route, eth0Name, err)
		}
		// 我们变更路由主要是将路由条目的`dev`字段改为bridgeName接口.
		// 而在route对象中, LinkIndex才是表示
		route.LinkIndex = linkBridge.Attrs().Index
		err = netlink.RouteAdd(&route)
		if err != nil {
			klog.Errorf("failed to add route %+v to %s: %s", route, bridgeName, err)
			return
		}
	}

	return
}

// cmdAdd: 在调用此函数时, 以由kubelet创建好pause容器, 正是需要为其部署网络的时候.
// 而对应的业务容器此时还未创建.
func cmdAdd(args *skel.CmdArgs) (err error) {
	klog.V(3).Infof("cmdAdd args: %+v", args)
	netConf := &NetConf{}
	err = json.Unmarshal(args.StdinData, netConf)
	if err != nil {
		return
	}
	delegateBytes, err := json.Marshal(netConf.Delegate)
	if err != nil {
		return
	}
	// 放在dhcp和bridge调用之前是有目的的, 看函数注释.
	err = linkMasterBridge()
	if err != nil {
		return
	}

	/////////////////////////////////
	ctx := context.TODO()
	go startDHCP(ctx)
	klog.V(3).Info("run dhcp plugin success")

	result, err := invoke.DelegateAdd(ctx, netConf.Delegate["type"].(string), delegateBytes, nil)
	if err != nil {
		klog.Errorf("faliled to run bridge plugin: %s", err)
		return
	}
	klog.V(3).Infof("run bridge plugin success: %s", result.Print())

	return
}

func cmdDel(args *skel.CmdArgs) error {
	return nil
}

func cmdCheck(args *skel.CmdArgs) error {
	// TODO: implement
	return nil
}

func main() {
	klog.V(3).Info("start cni-terway plugin......")
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, versionAll, "cni-terway")
}
