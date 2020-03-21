package dhcp

import (
	"context"
	"os"

	"k8s.io/klog"

	"github.com/generals-space/cni-terway/util"
)

// StartDHCP 运行dhcp插件, 作为守护进程.
func StartDHCP(ctx context.Context, binPath, sockPath, logPath string) (proc *os.Process, err error) {
	if util.Exists(sockPath) {
		klog.Info("dhcp.sock already exist")
		return
	}
	klog.Info("dhcp.sock doesn't exist, continue.")

	/*
		// 放弃粗暴地移除sock文件
		err = os.Remove(sockPath)
		if err != nil {
			if err.Error() != "remove /run/cni/dhcp.sock: no such file or directory" {
				klog.Errorf("try to rm dhcp.sock failed: %s", err)
				return
			}
			// 目标不存在, 则继续.
		}
	*/
	if os.Getppid() != 1 {
		args := []string{binPath, "daemon"}

		/*
			procAttr := &os.ProcAttr{
				Files: []*os.File{
					os.Stdin,
					os.Stdout,
					os.Stderr,
				},
			}
		*/
		logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			klog.Errorf("create dhcp log file failed: %s", err)
			return nil, err
		}
		procAttr := &os.ProcAttr{
			Files: []*os.File{
				logFile,
				logFile,
				logFile,
			},
		}
		// os.StartProcess()也是非阻塞函数, 运行时立刻返回(proc进程对象会创建好),
		// 然后如果目标子进程运行出错, 就会返回到err处理部分.
		proc, err := os.StartProcess(binPath, args, procAttr)
		if err != nil || proc == nil || proc.Pid <= 0 {
			klog.Errorf("dhcp start failed: %s", err)
			// 即使执行失败, 打印完后也不退出, 除非显式调用return
			return nil, err
		}
		// 如果这里执行完, 发现目标进程启动失败, 会回到上面err处理部分.
		klog.Infof("dhcp daemon started, proc: %+v", proc)
		return proc, nil
	}
	return
}

// StopDHCP Pod退出时清理dhcp子进程及sock文件
func StopDHCP(proc *os.Process, sockPath string) (err error) {
	if proc != nil && proc.Pid >= 0 {
		err = proc.Kill()
		if err != nil {
			klog.Errorf("dhcp process kill failed: %s", err)
			return
		}
	}
	err = os.Remove(sockPath)
	if err != nil {
		klog.Errorf("dhcp sock remove failed: %s", err)
		return
	}
	return
}
