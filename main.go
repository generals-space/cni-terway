package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/vishvananda/netlink"
	"k8s.io/klog"

	"github.com/generals-space/cni-terway/util"
)

const (
	bridgeName = "cnibr0"
	eth0Name   = "ens160"
	dhcpPath   = "/opt/cni/bin/dhcp"
	dhcpSock   = "/run/cni/dhcp.sock"
)

// CmdOpts 命令行参数对象
type CmdOpts struct {
	bridgeName string
	eth0Name   string
}

var cmdFlags = flag.NewFlagSet("cni-terway", flag.ExitOnError)
var cmdOpts = CmdOpts{}

func init() {
	cmdFlags.StringVar(&cmdOpts.eth0Name, "iface", "eth0", "the network interface using to communicate with kubernetes cluster")
	cmdFlags.StringVar(&cmdOpts.bridgeName, "bridge", "cnibr0", "this plugin will create a bridge device, named by this option")
	cmdFlags.Parse(os.Args[1:])
}

// startDHCP 运行dhcp插件, 作为守护进程.
func startDHCP(ctx context.Context) (err error) {
	if util.Exists(dhcpSock) {
		klog.Info("dhcp.sock already exist")
		return
	}
	klog.Info("dhcp.sock doesn't exist, continue.")

	/*
		// 放弃粗暴地移除sock文件
		err = os.Remove(dhcpSock)
		if err != nil {
			if err.Error() != "remove /run/cni/dhcp.sock: no such file or directory" {
				klog.Errorf("try to rm dhcp.sock failed: %s", err)
				return
			}
			// 目标不存在, 则继续.
		}
	*/
	if os.Getppid() != 1 {
		args := []string{dhcpPath, "daemon"}
		procAttr := &os.ProcAttr{
			Files: []*os.File{
				os.Stdin,
				os.Stdout,
				os.Stderr,
			},
		}
		// os.StartProcess()也是非阻塞函数, 运行时立刻返回(proc进程对象会创建好),
		// 然后如果目标子进程运行出错, 就会返回到err处理部分.
		proc, err := os.StartProcess(dhcpPath, args, procAttr)
		if err != nil {
			klog.Errorf("dhcp start failed: %s", err)
			// 即使执行失败, 打印完后也不退出, 除非显式调用return
			return err
		}
		// 如果这里执行完, 发现目标进程启动失败, 会回到上面err处理部分.
		klog.Infof("dhcp daemon started, proc: %+v", proc)
	}
	return
}

// 手动创建 cnibr0 接口, 然后将eth0接入, 因为如果不完成接入,
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

func main() {
	klog.Info("start cni-terway plugin......")
	klog.V(3).Infof("cmd opt: %+v", cmdOpts)
	var err error

	// 创建bridge接口, 部署桥接网络.
	err = linkMasterBridge()
	if err != nil {
		return
	}
	klog.Info("link bridge success")

	/////////////////////////////////
	ctx := context.TODO()
	err = startDHCP(ctx)
	if err != nil {
		klog.Errorf("faliled to run dhcp plugin: %s", err)
		return
	}
	klog.Info("run dhcp plugin success")

	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	klog.Info(<-sigCh)
	return
}
