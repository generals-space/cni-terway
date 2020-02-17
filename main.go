package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	"github.com/vishvananda/netlink"
	"k8s.io/klog"

	"github.com/generals-space/cni-terway/netconf"
	"github.com/generals-space/cni-terway/pkg"
)

var (
	cmdOpts        CmdOpts
	cmdFlags       = flag.NewFlagSet("cni-terway", flag.ExitOnError)
	dhcpBinPath    = "/opt/cni/bin/dhcp"
	dhcpSockPath   = "/run/cni/dhcp.sock"
	dhcpLogPath    = "/run/cni/dhcp.log"
	dhcpProc       *os.Process
	cniNetConfPath = "/etc/cni/net.d/10-cni-terway.conf"
)

// CmdOpts 命令行参数对象
type CmdOpts struct {
	bridgeName string
	eth0Name   string
}

func init() {
	cmdFlags.StringVar(&cmdOpts.eth0Name, "iface", "eth0", "the network interface using to communicate with kubernetes cluster")
	cmdFlags.StringVar(&cmdOpts.bridgeName, "bridge", "cnibr0", "this plugin will create a bridge device, named by this option")
	cmdFlags.Parse(os.Args[1:])
}

// linkMasterBridge 手动创建 cnibr0 接口, 然后将eth0接入, 因为如果不完成接入,
// invoke调用bridge+dhcp插件时请求会失败, 出现如下报错
// error calling DHCP.Allocate: no more tries
func linkMasterBridge() (err error) {
	linkBridge, err := netlink.LinkByName(cmdOpts.bridgeName)
	if err != nil {
		if err.Error() != "Link not found" {
			klog.Errorf("failed to get bridge %s: %s.", cmdOpts.bridgeName, err)
			return
		}
		klog.Warningf("bridge: %s doesn't exist, try to create it manually.", cmdOpts.bridgeName)
		// 尝试手动创建然后启动
		bridge := &netlink.Bridge{
			LinkAttrs: netlink.LinkAttrs{
				Name: cmdOpts.bridgeName,
			},
		}
		err = netlink.LinkAdd(bridge)
		if err != nil {
			klog.Warningf("failed to craete bridge %s: %s", cmdOpts.bridgeName, err)
			return
		}
		err = netlink.LinkSetUp(bridge)
		if err != nil {
			klog.Warningf("failed to setup bridge %s: %s", cmdOpts.bridgeName, err)
			return
		}
		linkBridge, err = netlink.LinkByName(cmdOpts.bridgeName)
		if err != nil {
			klog.Warningf("failed to get bridge %s: %s", cmdOpts.bridgeName, err)
			return
		}
	}

	linkEth0, err := netlink.LinkByName(cmdOpts.eth0Name)
	if err != nil {
		klog.Errorf("failed to get target device %s: %s", cmdOpts.eth0Name, err)
		return err
	}
	addrs, err := netlink.AddrList(linkEth0, netlink.FAMILY_V4)
	if err != nil {
		klog.Errorf("failed to get address list on %s: %s", cmdOpts.eth0Name, err)
		return
	}
	klog.V(3).Infof("get addresses of device %s, len: %d: %+v", cmdOpts.eth0Name, len(addrs), addrs)

	// 之后要由bridge设备接管eth0设备的所有路由
	routes, err := netlink.RouteList(linkEth0, netlink.FAMILY_V4)
	if err != nil {
		klog.Errorf("failed to get address list on %s: %s", cmdOpts.eth0Name, err)
		return
	}
	klog.V(3).Infof("get route items of device %s, len: %d: %+v", cmdOpts.eth0Name, len(routes), routes)

	err = netlink.LinkSetMaster(linkEth0, linkBridge)
	if err != nil {
		return err
	}
	klog.V(3).Infof("set %s master %s success", cmdOpts.eth0Name, cmdOpts.bridgeName)

	for _, addr := range addrs {
		// 从一个接口上移除IP地址, 对应的路由应该也会被移除.
		err = netlink.AddrDel(linkEth0, &addr)
		if err != nil {
			klog.Errorf("failed to del addr on %s: %s", cmdOpts.eth0Name, err)
			continue
		}
		// 不知道Label是什么意思, 但是如果不修改这个字段, 会出现如下问题.
		// label must begin with interface name
		// 在AddrAdd()源码中有检验Label是否以接口名称为前缀的判断.
		addr.Label = cmdOpts.bridgeName
		err = netlink.AddrAdd(linkBridge, &addr)
		if err != nil {
			klog.Errorf("failed to add addr to %s: %s", cmdOpts.bridgeName, err)
			continue
		}
	}
	klog.V(3).Infof("move address from %s to %s success", cmdOpts.eth0Name, cmdOpts.bridgeName)

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
			klog.Errorf("failed to del route %+v from: %s", route, cmdOpts.eth0Name, err)
		}
		// 我们变更路由主要是将路由条目的`dev`字段改为cmdOpts.bridgeName接口.
		// 而在route对象中, LinkIndex才是表示
		route.LinkIndex = linkBridge.Attrs().Index
		err = netlink.RouteAdd(&route)
		if err != nil {
			klog.Errorf("failed to add route %+v to %s: %s", route, cmdOpts.bridgeName, err)
			return
		}
	}

	return
}

func fillNetConf() (err error) {
	netConfContent, err := ioutil.ReadFile(cniNetConfPath)
	if err != nil {
		klog.Errorf("failed to read cni netconf file: %s", err)
		return
	}

	netConf := &netconf.NetConf{}
	err = json.Unmarshal(netConfContent, netConf)
	if err != nil {
		klog.Errorf("failed to unmarshal cni netconf content: %s", err)
		return
	}

	serviceIPCIDR, err := pkg.GetServiceIPCIDR()
	if err != nil {
		klog.Errorf("failed to get service ip cidr: %s", err)
		return
	}
	klog.Infof("get service ip cidr: %s", serviceIPCIDR)
	netConf.ServiceIPCIDR = serviceIPCIDR

	netConfContent, err = json.Marshal(netConf)
	if err != nil {
		klog.Errorf("failed to marshal cni netconf: %s", err)
		return
	}

	err = ioutil.WriteFile(cniNetConfPath, netConfContent, 0644)
	if err != nil {
		klog.Errorf("failed to write into cni netconf: %s", err)
		return
	}

	return
}

func main() {
	klog.Info("start cni-terway plugin......")
	klog.V(3).Infof("cmd opt: %+v", cmdOpts)
	var err error

	err = fillNetConf()
	if err != nil {

	}

	// 创建bridge接口, 部署桥接网络.
	err = linkMasterBridge()
	if err != nil {
		return
	}
	klog.Info("link bridge success")

	/////////////////////////////////
	ctx := context.TODO()
	dhcpProc, err = pkg.StartDHCP(ctx, dhcpBinPath, dhcpSockPath, dhcpLogPath)
	if err != nil {
		klog.Errorf("faliled to run dhcp plugin: %s", err)
		return
	}
	klog.Info("run dhcp plugin success")

	// 使用两个channel, sigCh接收信号, 之后的清理操作有可能失败, 失败后不能直接退出.
	// 退出的时机由doneCh决定.
	sigCh := make(chan os.Signal, 1)
	doneCh := make(chan bool, 1)
	// 一般delete pod时, 收到的是SIGTERM信号.
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for sig := range sigCh {
			klog.Infof("receive signal %d", sig)
			if sig != syscall.SIGTERM {
				continue
			}

			err := pkg.StopDHCP(dhcpProc, dhcpSockPath)
			if err != nil {
				klog.Errorf("receive SIGTERM, but stop dhcp process failed: %s", err)
				continue
			}
			doneCh <- true
		}
	}()
	<-doneCh

	klog.Info("exiting")
	return
}
