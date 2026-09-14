package fs

import "os"

// 创建文件夹

func MkDir(path string) error {
	if FileExists(path) {
		return nil
	}

	return os.Mkdir(path, 0755)
}
