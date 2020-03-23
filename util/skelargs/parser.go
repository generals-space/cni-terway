package skelargs

import (
	"errors"
	"fmt"
	"strings"
)

// ParseValueFromArgs 从 cmdAdd/cmdDel 传入的 skel.CmdArgs 参数中, 
// Args成员字符串中解析中指定字段的值.
// CNI_ARGS=IgnoreUnknown=1;K8S_POD_NAMESPACE=default;K8S_POD_NAME=test-ds-2hxbm;K8S_POD_INFRA_CONTAINER_ID=f02a00be2994c8def9d3fd06b1c2707ae2677e135326f1ef788f1b12e23bd345 KUBELET_CONFIG_ARGS=--config=/var/lib/kubelet/config.yaml 
func ParseValueFromArgs(key, argString string) (string, error) {
	if argString == "" {
		return "", errors.New("CNI_ARGS is required")
	}
	args := strings.Split(argString, ";")
	for _, arg := range args {
		if strings.HasPrefix(arg, fmt.Sprintf("%s=", key)) {
			podName := strings.TrimPrefix(arg, fmt.Sprintf("%s=", key))
			if len(podName) > 0 {
				return podName, nil
			}
		}
	}
	return "", fmt.Errorf("%s is required in CNI_ARGS", key)
}
