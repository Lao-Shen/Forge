package process

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// 打开一个进程

const (
	QueryAccess     = windows.PROCESS_QUERY_INFORMATION
	TerminateAccess = windows.PROCESS_TERMINATE
)

func OpenProcess(pid uint32, access uint32) (*Process, error) {
	handle, err := windows.OpenProcess(
		access,
		false,
		pid,
	)
	if err != nil {
		return nil, err
	}

	var buf [windows.MAX_PATH]uint16
	size := uint32(len(buf))

	err = windows.QueryFullProcessImageName(
		handle,
		0,
		&buf[0],
		&size,
	)

	if err != nil {
		return nil, err
	}

	return &Process{
		handle:   handle,
		pid:      pid,
		fullPath: windows.UTF16ToString(buf[:size]),
	}, nil
}

func (p *Process) Close() error {
	return windows.CloseHandle(p.handle)
}

func (p *Process) Pid() uint32 {
	return p.pid
}

func (p *Process) Name() string {
	return filepath.Base(p.fullPath)
}

func (p *Process) FullPath() string {
	return p.fullPath
}

func (p *Process) Wait() error {
	ret, err := windows.WaitForSingleObject(p.handle, windows.INFINITE)
	if err != nil {
		return err
	}

	if ret != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("WaitForSingleObject returned %d", ret)
	}

	return nil
}

func (p *Process) Username() (string, error) {
	var token windows.Token

	err := windows.OpenProcessToken(
		p.handle,
		windows.TOKEN_QUERY,
		&token)

	if err != nil {
		return "", err
	}

	defer token.Close()

	tokenUser, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}

	account, domain, _, err := tokenUser.User.Sid.LookupAccount("")
	if err != nil {
		return "", err
	}

	if domain != "" {
		account = domain + `\` + account
	}

	return account, nil
}

func (p *Process) UserInfo() (*User, error) {
	var token windows.Token

	err := windows.OpenProcessToken(
		p.handle,
		windows.TOKEN_QUERY,
		&token)

	if err != nil {
		return nil, err
	}

	defer token.Close()

	tokenUser, err := token.GetTokenUser()
	if err != nil {
		return nil, err
	}

	account, domain, _, err := tokenUser.User.Sid.LookupAccount("")
	if err != nil {
		return nil, err
	}

	sid := tokenUser.User.Sid.String()

	return &User{
		name:   account,
		domain: domain,
		sid:    sid,
	}, nil
}

func (p *Process) Kill() error {
	return windows.TerminateProcess(p.handle, 1)
}
