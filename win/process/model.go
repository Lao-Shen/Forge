package process

import "golang.org/x/sys/windows"

// 进程数据结构

type Process struct {
	pid      uint32
	handle   windows.Handle
	fullPath string
}

type User struct {
	name   string
	sid    string
	domain string
}
