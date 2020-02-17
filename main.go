package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

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
	err = pkg.InstallBridgeNetwork(cmdOpts.bridgeName, cmdOpts.eth0Name)
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
			var err error
			klog.Infof("receive signal %d", sig)
			if sig != syscall.SIGTERM {
				continue
			}

			err = pkg.StopDHCP(dhcpProc, dhcpSockPath)
			if err != nil {
				klog.Errorf("receive SIGTERM, but stop dhcp process failed: %s", err)
				continue
			}
			
			err = pkg.UninstallBridgeNetwork(cmdOpts.bridgeName, cmdOpts.eth0Name)
			if err != nil {
				klog.Errorf("receive SIGTERM, but uninstall bridge network failed, you should check it: %s", err)
				continue
			}
			doneCh <- true
		}
	}()
	<-doneCh

	klog.Info("exiting")
	return
}
