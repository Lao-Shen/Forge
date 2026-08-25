package peicon

// 本文件的 .rsrc 资源树遍历逻辑与 PE 图标资源结构体抄自 orcastor/fico
// （https://github.com/orcastor/fico，MIT License, Copyright (c) 2024 orcastor），
// 仅做了两处调整：资源 ID 解析失败时跳过、把 pe.File 传进来以便按节表定位数据。

import (
	"bytes"
	"encoding/binary"
	"strconv"
	"strings"
	"unicode/utf16"
)

const (
	sectionResources = ".rsrc"
	rtIcon           = "3/"  // RT_ICON：单张图像，载荷是 DIB 或 PNG
	rtGroupIcon      = "14/" // RT_GROUP_ICON：图标组目录，一个组 = 一个 .ico
)

// resource 是资源树里的一个叶子。Name 形如 "<type>/<name>/<language>"，
// 例如 "3/1/2052"（ID 命名）或 "14/IDR_MAINFRAME/1033"（字符串命名）。
type resource struct {
	Name string
	Data []byte
}

// ICONDIR 是 .ico 文件头，也是 RT_GROUP_ICON 资源的头部。
// https://en.wikipedia.org/wiki/ICO_(file_format)#Header
type ICONDIR struct {
	Reserved uint16 // 保留字段，必须为 0
	Type     uint16 // 图标类型，必须为 1
	Count    uint16 // 图标数量
}

// IconCommon 是 .ico 目录项与 RT_GROUP_ICON 目录项共有的前 12 个字节。
type IconCommon struct {
	Width      uint8  // 宽度，像素；0 表示 256
	Height     uint8  // 高度，像素；0 表示 256
	Color      uint8  // 色深；0 表示 256 色及以上
	Reserved   uint8  // 保留字段
	Planes     uint16 // 颜色平面数
	BitCount   uint16 // 每像素位数
	BytesInRes uint32 // 图像数据大小
}

// RESDIR 是 RT_GROUP_ICON 里的目录项，末尾是 RT_ICON 的资源 ID。
type RESDIR struct {
	IconCommon
	ID uint16
}

// ICONDIRENTRY 是 .ico 文件里的目录项，末尾是图像数据在文件中的偏移。
type ICONDIRENTRY struct {
	IconCommon
	Offset uint32
}

// parseDir 递归解析 IMAGE_RESOURCE_DIRECTORY。b 是 .rsrc 节的原始数据，
// p 是当前目录在 b 中的偏移，prefix 是已累积的 "<type>/<name>/" 路径，
// addr 是 .rsrc 节的虚拟地址（用于把 RVA 换算成节内偏移）。
// 只下探 RT_ICON 和 RT_GROUP_ICON 两棵子树。
func parseDir(b []byte, p int, prefix string, addr uint32) []*resource {
	if prefix != "" && !strings.HasPrefix(prefix, rtIcon) && !strings.HasPrefix(prefix, rtGroupIcon) {
		return nil
	}
	if p < 0 || p+16 > len(b) {
		return nil
	}

	le := binary.LittleEndian

	var res []*resource
	// 跳过 Characteristics、Timestamp、Major、Minor，取具名项数 + ID 项数
	n := int(le.Uint16(b[p+12:p+14])) + int(le.Uint16(b[p+14:p+16]))
	if max := (len(b) - p - 16) / 8; n > max {
		n = max
	}

	for i := 0; i < n; i++ {
		o := 8*i + p + 16
		name := le.Uint32(b[o : o+4])
		offsetToData := le.Uint32(b[o+4 : o+8])
		path := prefix
		if name&0x80000000 > 0 { // 高位置位表示这是字符串命名
			dirStr := int(name & 0x7FFFFFFF)
			if dirStr < 0 || dirStr+2 > len(b) {
				continue
			}
			length := int(le.Uint16(b[dirStr : dirStr+2]))
			if dirStr+2+length<<1 > len(b) {
				continue
			}
			resID := make([]uint16, length)
			binary.Read(bytes.NewReader(b[dirStr+2:dirStr+2+length<<1]), le, resID)
			path += string(utf16.Decode(resID))
		} else { // ID 命名
			path += strconv.Itoa(int(name))
		}

		if offsetToData&0x80000000 > 0 { // 高位置位表示指向下一层目录
			res = append(res, parseDir(b, int(offsetToData&0x7FFFFFFF), path+"/", addr)...)
			continue
		}

		// 叶子，指向 IMAGE_RESOURCE_DATA_ENTRY
		if int(offsetToData)+8 > len(b) {
			continue
		}
		offset := int(le.Uint32(b[offsetToData : offsetToData+4]))
		length := int(le.Uint32(b[offsetToData+4 : offsetToData+8]))

		// IMAGE_RESOURCE_DATA_ENTRY 里的偏移是 RVA，减去节的虚拟地址得到节内偏移
		offset -= int(addr)

		if offset < 0 || length < 0 || offset+length > len(b) {
			continue
		}

		res = append(res, &resource{Name: path, Data: b[offset : offset+length]})
	}
	return res
}
