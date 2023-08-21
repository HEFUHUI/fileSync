package utils

import (
	"os"
	"path"
)

// GetSubDir 获取子目录列表
func GetSubDir(dir string) []string {
	var result []string
	fileInfo, err := os.ReadDir(dir)
	if err != nil {
		return result
	}
	for _, v := range fileInfo {
		if v.IsDir() {
			result = append(result, path.Join(dir, v.Name()))
			result = append(result, GetSubDir(path.Join(dir, v.Name()))...)
		}
	}
	return result
}
