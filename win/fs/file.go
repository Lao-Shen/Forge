package fs

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

const (
	All        = 0
	OnlyFile   = 1
	OnlyFolder = 2
)

type File struct {
	fullPath string
	size     uint64
	isDir    bool
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func GetFile(fullPath string) (*File, error) {
	stat, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		return &File{
			fullPath: fullPath,
			size:     0,
			isDir:    true,
		}, nil
	}

	return &File{
		fullPath: fullPath,
		size:     uint64(stat.Size()),
		isDir:    false,
	}, nil
}

func (f *File) Name() string {
	return filepath.Base(f.fullPath)
}

func (f *File) Size() uint64 {
	return f.size
}

func (f *File) FullPath() string {
	return f.fullPath
}

func (f *File) Ext() string {
	return filepath.Ext(f.fullPath)
}

// todo

type MoveOption struct {
	Replace bool // 替换已存在的
	Clean   bool // 移动文件夹时，如果移动位置的文件夹已存在，是否清空那个文件夹
}

// Move : move to dist , replace : if dist is existed, replace it!
func (f *File) Move(dist string, replace bool) error {
	srcPtr, err := windows.UTF16PtrFromString(f.fullPath)
	if err != nil {
		return err
	}

	destPtr, err := windows.UTF16PtrFromString(dist)
	if err != nil {
		return err
	}

	flags := uint32(windows.MOVEFILE_COPY_ALLOWED)
	if replace {
		flags |= windows.MOVEFILE_REPLACE_EXISTING
	}

	return windows.MoveFileEx(
		srcPtr,
		destPtr,
		flags,
	)
}

/*
1.确认在同一个分组
2.如果是文件夹是否需要先删除
3.复制
*/

func moveInSamplePart(src, dist string, option MoveOption) error {
	if !inSamplePart(src, dist) {
		return fmt.Errorf("%s and %s not in sample part", src, dist)
	}

	if option.Clean {
		// 删除
		err := os.RemoveAll(dist)
		if err != nil {
			return err
		}

		return os.Rename(src, dist)
	}

	flags := uint32(windows.MOVEFILE_COPY_ALLOWED)
	if option.Replace {
		flags |= windows.MOVEFILE_REPLACE_EXISTING
	}

	srcPtr, err := windows.UTF16PtrFromString(src)
	if err != nil {
		return err
	}

	destPtr, err := windows.UTF16PtrFromString(dist)
	if err != nil {
		return err
	}

	return windows.MoveFileEx(srcPtr, destPtr, flags)
}

// 两个路径是否在通过一个分区
func inSamplePart(src, dist string) bool {
	return filepath.VolumeName(src) == filepath.VolumeName(dist)
}

// copy to
