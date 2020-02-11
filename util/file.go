package util

import (
	"os"
)

// Exists 判断所给路径文件/文件夹是否存在, 返回true/false.
func Exists(path string) bool {
	//os.Stat获取文件信息
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}
