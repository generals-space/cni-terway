package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"

	"k8s.io/klog"

	"github.com/generals-space/cni-terway/pkg/bridge"
	"github.com/generals-space/cni-terway/pkg/config"
	"github.com/generals-space/cni-terway/pkg/dhcp"
	"github.com/generals-space/cni-terway/pkg/serviceipcidr"
	"github.com/generals-space/cni-terway/pkg/signals"
)

var (
	cmdOpts        config.CmdOpts
	cmdFlags       = flag.NewFlagSet("cni-terway", flag.ExitOnError)
	dhcpBinPath    = "/opt/cni/bin/dhcp"
	dhcpSockPath   = "/run/cni/dhcp.sock"
	dhcpLogPath    = "/run/cni/dhcp.log"
	dhcpProc       *os.Process
	cniNetConfPath = "/etc/cni/net.d/10-cni-terway.conf"
)

func init() {
	cmdFlags.StringVar(&cmdOpts.Eth0Name, "iface", "eth0", "the network interface using to communicate with kubernetes cluster")
	cmdFlags.StringVar(&cmdOpts.BridgeName, "bridge", "cnibr0", "this plugin will create a bridge device, named by this option")
	cmdFlags.Parse(os.Args[1:])
}

func fillNetConf() (err error) {
	netConfContent, err := ioutil.ReadFile(cniNetConfPath)
	if err != nil {
		klog.Errorf("failed to read cni netconf file: %s", err)
		return
	}

	netConf := &config.NetConf{}
	err = json.Unmarshal(netConfContent, netConf)
	if err != nil {
		klog.Errorf("failed to unmarshal cni netconf content: %s", err)
		return
	}

	serviceIPCIDR, err := serviceipcidr.GetServiceIPCIDR()
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

// stopHandler 执行退出时的清理操作, 如停止dhcp进程, 恢复原本的网络拓扑等.
func stopHandler(cmdOpts *config.CmdOpts, doneCh chan<- bool) {
	var err error
	klog.Infof("receive stop signal")

	err = dhcp.StopDHCP(dhcpProc, dhcpSockPath)
	if err != nil {
		klog.Errorf("receive signal, but stop dhcp process failed: %s", err)
	}

	err = bridge.UninstallBridgeNetwork(cmdOpts.BridgeName, cmdOpts.Eth0Name)
	if err != nil {
		klog.Errorf("receive signal, but uninstall bridge network failed, you should check it: %s", err)
	}
	doneCh <- true
}

func main() {
	klog.Info("start cni-terway plugin......")
	klog.V(3).Infof("cmd opt: %+v", cmdOpts)
	var err error

	err = fillNetConf()
	if err != nil {
		return
	}

	// 创建bridge接口, 部署桥接网络, 使bridge设备接管宿主机主网卡的功能.
	// 虽然即使不事先创建bridge接口, 在cni部分调用bridge插件也会自动创建,
	// 但是由于bridge插件在创建bridge设备的同时就会调用dhcp, dhcp请求会无法正确发出.
	err = bridge.InstallBridgeNetwork(cmdOpts.BridgeName, cmdOpts.Eth0Name)
	if err != nil {
		return
	}
	klog.Info("link bridge success")

	/////////////////////////////////
	ctx := context.TODO()
	dhcpProc, err = dhcp.StartDHCP(ctx, dhcpBinPath, dhcpSockPath, dhcpLogPath)
	if err != nil {
		klog.Errorf("faliled to run dhcp plugin: %s", err)
		return
	}
	klog.Info("run dhcp plugin success")

	// 退出的时机由doneCh决定.
	doneCh := make(chan bool, 1)
	signals.SetupSignalHandler(stopHandler, &cmdOpts, doneCh)
	<-doneCh

	klog.Info("exiting")
	return
}
