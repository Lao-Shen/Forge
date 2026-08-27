package process

import (
	"testing"
)

func TestOpenProcess(t *testing.T) {
	var pid uint32 = 21548

	process, err := OpenProcess(pid, TerminateAccess|QueryAccess)
	if err != nil {
		t.Fatal(err.Error())
		return
	}

	t.Log(process.Name())
	t.Log(process.FullPath())
	t.Log(process.Pid())
	t.Log(process.UserInfo())
	t.Log(process.Kill())
	t.Log(process.Close())
}
