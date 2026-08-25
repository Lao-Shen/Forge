// Package peicon 从 Windows PE 文件（exe、dll、mui、mun）中提取图标。
//
// 两个典型用法：ExtractAll 导出全部图标，ExtractSize 按目标像素导出单张 PNG。
//
// PE 里的图标由两类资源组成：RT_GROUP_ICON 是「图标组」，相当于一个 .ico 的目录；
// RT_ICON 是组内的单张图像，载荷要么是 DIB 位图，要么是 PNG 流（Vista 起 256×256 常用）。
// 同一个组里两种编码可以混存。
package peicon

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/image/draw"
)

// Group 是一个 RT_GROUP_ICON 资源，对应一个完整的多尺寸 .ico。
type Group struct {
	Index  int     // 枚举序号，同一文件多次调用保持稳定
	Name   string  // 资源名，如 "1" 或 "IDR_MAINFRAME"
	Images []Image // 组内各尺寸
}

// Image 是组内的一张图像。Data 是原始资源字节，导出 .ico 时逐字节透传，不做重编码。
type Image struct {
	ResID    uint16 // RT_ICON 资源 ID
	Width    int    // 真实像素宽（组目录里的 0 已归一化为 256）
	Height   int    // 真实像素高
	BitDepth int    // 每像素位数
	Format   string // "dib" 或 "png"
	Data     []byte // 原始资源字节
}

// List 枚举 path 中的所有图标组，只读头部，不解码像素。
//
// 没有图标是合法结果，返回空切片而非错误 —— Win10 1903 起 notepad.exe、
// imageres.dll 等系统文件的图标已迁移到 C:\Windows\SystemResources\*.mun。
func List(path string) ([]Group, error) {
	f, err := pe.Open(path)
	if err != nil {
		return nil, fmt.Errorf("peicon: 打开 PE 文件失败: %w", err)
	}
	defer f.Close()

	rsrc := f.Section(sectionResources)
	if rsrc == nil {
		return nil, nil
	}
	data, err := rsrc.Data()
	if err != nil {
		return nil, fmt.Errorf("peicon: 读取 .rsrc 节失败: %w", err)
	}

	idmap := make(map[uint16]*resource)
	var groupRes []*resource
	for _, r := range parseDir(data, 0, "", rsrc.VirtualAddress) {
		switch {
		case strings.HasPrefix(r.Name, rtGroupIcon):
			groupRes = append(groupRes, r)
		case strings.HasPrefix(r.Name, rtIcon):
			id, err := strconv.ParseUint(nameSegment(r.Name), 10, 16)
			if err != nil {
				continue // 非数字命名的 RT_ICON，组目录引用不到它
			}
			idmap[uint16(id)] = r
		}
	}

	groups := make([]Group, 0, len(groupRes))
	for _, r := range groupRes {
		entries, err := parseGroupDir(r.Data)
		if err != nil || len(entries) == 0 {
			continue
		}
		g := Group{Index: len(groups), Name: nameSegment(r.Name)}
		for _, e := range entries {
			res, ok := idmap[e.ID]
			if !ok {
				continue // 组目录引用了不存在的 RT_ICON
			}
			g.Images = append(g.Images, newImage(e, res.Data))
		}
		if len(g.Images) > 0 {
			groups = append(groups, g)
		}
	}
	return groups, nil
}

// ExtractAll 导出 path 中的全部图标到 outDir，返回实际写出的文件路径。
//
// 每个图标组一个子目录，内含 icon.ico（含该组全部尺寸）以及每个尺寸的 PNG。
// 同一组里出现多张相同像素尺寸时（不同色深），PNG 文件名追加 _id<资源ID> 加以区分。
//
// 单张图像解码失败只跳过该 PNG，不中断整体导出 —— 它的原始数据仍然完整保留在 icon.ico 里。
func ExtractAll(path, outDir string) ([]string, error) {
	groups, err := List(path)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("peicon: %s 中没有图标资源", path)
	}

	var written []string
	for _, g := range groups {
		dir := filepath.Join(outDir, fmt.Sprintf("%d_%s", g.Index, sanitizeName(g.Name)))
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return written, fmt.Errorf("peicon: 创建目录失败: %w", err)
		}

		icoPath := filepath.Join(dir, "icon.ico")
		if err := saveICO(g.Images, icoPath); err != nil {
			return written, err
		}
		written = append(written, icoPath)

		// 先统计各像素尺寸出现次数，出现多次的才需要 _id 后缀
		dup := make(map[[2]int]int, len(g.Images))
		for _, im := range g.Images {
			dup[[2]int{im.Width, im.Height}]++
		}

		for _, im := range g.Images {
			img, err := decodeImage(im)
			if err != nil {
				continue
			}
			name := fmt.Sprintf("%dx%d", im.Width, im.Height)
			if dup[[2]int{im.Width, im.Height}] > 1 {
				name += fmt.Sprintf("_id%d", im.ResID)
			}
			p := filepath.Join(dir, name+".png")
			if err := savePNG(img, p); err != nil {
				return written, err
			}
			written = append(written, p)
		}
	}
	return written, nil
}

// ExtractSize 从 path 中挑出最匹配 w×h 的一张图标，缩放到恰好 w×h 后存为 PNG。
//
// 挑选顺序：尺寸完全相等 > 两个维度都不小于目标（只缩不放）> 其余；
// 同级内取尺寸差最小者，再取色深最高者。跨全部图标组择优。
func ExtractSize(path string, w, h int, dstPath string) error {
	if w <= 0 || h <= 0 {
		return fmt.Errorf("peicon: 目标尺寸必须为正数，收到 %dx%d", w, h)
	}

	groups, err := List(path)
	if err != nil {
		return err
	}
	var all []Image
	for _, g := range groups {
		all = append(all, g.Images...)
	}
	if len(all) == 0 {
		return fmt.Errorf("peicon: %s 中没有图标资源", path)
	}

	best := pickBest(all, w, h)
	img, err := decodeImage(*best)
	if err != nil {
		return err
	}

	if img.Bounds().Dx() != w || img.Bounds().Dy() != h {
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
		img = dst
	}
	return savePNG(img, dstPath)
}

// pickBest 按「完全相等 > 只缩不放 > 需要放大」分级挑选最匹配的一张。
func pickBest(imgs []Image, w, h int) *Image {
	var best *Image
	bestClass, bestDist, bestDepth := 0, 0, 0
	for i := range imgs {
		im := &imgs[i]
		class := 2
		switch {
		case im.Width == w && im.Height == h:
			class = 0
		case im.Width >= w && im.Height >= h:
			class = 1
		}
		dist := abs(im.Width-w) + abs(im.Height-h)

		better := best == nil ||
			class < bestClass ||
			(class == bestClass && dist < bestDist) ||
			(class == bestClass && dist == bestDist && im.BitDepth > bestDepth)
		if better {
			best, bestClass, bestDist, bestDepth = im, class, dist, im.BitDepth
		}
	}
	return best
}

// newImage 用组目录里声明的尺寸兜底，再尝试从载荷解析真实尺寸覆盖。
//
// 组目录把 256 存成 0，绝不能拿这个字节直接参与比较 —— 一张 256×256 的 DIB
// 若被当成「宽度为 0 所以一定是 PNG」，解析必然失败并被判成 0×0，从而永远选不中。
func newImage(e RESDIR, data []byte) Image {
	im := Image{
		ResID:    e.ID,
		Width:    normDim(e.Width),
		Height:   normDim(e.Height),
		BitDepth: int(e.BitCount),
		Format:   sniffFormat(data),
		Data:     data,
	}
	if im.BitDepth == 0 {
		im.BitDepth = 32
	}
	switch im.Format {
	case "png":
		if cfg, err := png.DecodeConfig(bytes.NewReader(data)); err == nil {
			im.Width, im.Height = cfg.Width, cfg.Height
		}
	case "dib":
		if w, h, err := dibSize(data); err == nil {
			im.Width, im.Height = w, h
		}
	}
	return im
}

// decodeImage 把一张图标解码成 RGBA。
func decodeImage(im Image) (*image.RGBA, error) {
	switch im.Format {
	case "png":
		src, err := png.Decode(bytes.NewReader(im.Data))
		if err != nil {
			return nil, fmt.Errorf("peicon: 解码 PNG 图标 id=%d 失败: %w", im.ResID, err)
		}
		if rgba, ok := src.(*image.RGBA); ok {
			return rgba, nil
		}
		b := src.Bounds()
		dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
		return dst, nil
	case "dib":
		img, err := decodeDIB(im.Data)
		if err != nil {
			return nil, fmt.Errorf("peicon: 解码 DIB 图标 id=%d 失败: %w", im.ResID, err)
		}
		return img, nil
	}
	return nil, fmt.Errorf("peicon: 图标 id=%d 的载荷格式无法识别", im.ResID)
}

// parseGroupDir 解析 RT_GROUP_ICON 的目录：ICONDIR 头 + Count 个 14 字节的 RESDIR。
func parseGroupDir(d []byte) ([]RESDIR, error) {
	rd := bytes.NewReader(d)
	var hdr ICONDIR
	if err := binary.Read(rd, binary.LittleEndian, &hdr); err != nil {
		return nil, fmt.Errorf("peicon: 读取图标组目录失败: %w", err)
	}
	if int(hdr.Count) > rd.Len()/binary.Size(RESDIR{}) {
		return nil, fmt.Errorf("peicon: 图标组目录声明 %d 项但数据不足", hdr.Count)
	}
	entries := make([]RESDIR, hdr.Count)
	if err := binary.Read(rd, binary.LittleEndian, &entries); err != nil {
		return nil, fmt.Errorf("peicon: 读取图标组目录项失败: %w", err)
	}
	return entries, nil
}

// saveICO 把一组图像写成 .ico 文件。原始载荷（DIB 或 PNG）逐字节透传，不重编码。
func saveICO(imgs []Image, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return fmt.Errorf("peicon: 创建目录失败: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("peicon: 创建 ico 文件失败: %w", err)
	}
	defer f.Close()
	return writeICO(f, imgs)
}

func writeICO(w io.Writer, imgs []Image) error {
	if len(imgs) == 0 || len(imgs) > 0xFFFF {
		return fmt.Errorf("peicon: 图标数量 %d 不合法", len(imgs))
	}
	le := binary.LittleEndian

	hdr := make([]byte, 6)
	le.PutUint16(hdr[2:4], 1) // Type：1 表示图标
	le.PutUint16(hdr[4:6], uint16(len(imgs)))
	if _, err := w.Write(hdr); err != nil {
		return fmt.Errorf("peicon: 写入 ico 头失败: %w", err)
	}

	offset := 6 + 16*len(imgs)
	entry := make([]byte, 16)
	for _, im := range imgs {
		clear(entry)
		entry[0] = icoDim(im.Width)
		entry[1] = icoDim(im.Height)
		le.PutUint16(entry[4:6], 1) // Planes 固定为 1
		le.PutUint16(entry[6:8], uint16(im.BitDepth))
		le.PutUint32(entry[8:12], uint32(len(im.Data)))
		le.PutUint32(entry[12:16], uint32(offset))
		offset += len(im.Data)
		if _, err := w.Write(entry); err != nil {
			return fmt.Errorf("peicon: 写入 ico 目录项失败: %w", err)
		}
	}

	for _, im := range imgs {
		if _, err := w.Write(im.Data); err != nil {
			return fmt.Errorf("peicon: 写入 ico 图像数据失败: %w", err)
		}
	}
	return nil
}

// savePNG 编码并写盘，与 forge/image 包的做法保持一致。
func savePNG(img image.Image, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return fmt.Errorf("peicon: 创建目录失败: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("peicon: 创建 png 文件失败: %w", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("peicon: 编码 png 失败: %w", err)
	}
	return nil
}

// sniffFormat 按魔数判断 RT_ICON 载荷的编码。
func sniffFormat(d []byte) string {
	if len(d) >= 8 && string(d[:8]) == "\x89PNG\r\n\x1a\n" {
		return "png"
	}
	if len(d) >= 4 && binary.LittleEndian.Uint32(d[:4]) == dibHeaderLen {
		return "dib"
	}
	return ""
}

// nameSegment 从 "<type>/<name>/<lang>" 中取出 name 段。
func nameSegment(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// normDim 把组目录里的尺寸字节还原成像素值：0 表示 256。
func normDim(v uint8) int {
	if v == 0 {
		return 256
	}
	return int(v)
}

// icoDim 把像素值编码成 .ico 目录项的尺寸字节：256 及以上写 0。
func icoDim(v int) uint8 {
	if v >= 256 {
		return 0
	}
	return uint8(v)
}

// winReserved 是 Windows 保留的设备名，不能用作目录名。
var winReserved = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// sanitizeName 把资源名收拾成合法的目录名。调用方会额外加数字序号前缀，
// 所以这里返回空串也不会导致路径冲突。
func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
		if b.Len() >= 40 {
			break
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" || winReserved[strings.ToUpper(out)] {
		return "icon"
	}
	return out
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
