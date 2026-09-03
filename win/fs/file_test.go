package fs

import (
	"testing"
)

func TestNewFile(t *testing.T) {
	paths := []string{
		"E:\\Code",
		"E:\\Code\\Forge",
		"E:\\Code\\no",
		"E:\\Code\\traveler.zip",
	}

	for _, path := range paths {
		t.Log(FileExists(path))
	}
}
