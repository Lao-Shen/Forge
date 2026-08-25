package peicon

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"

	"golang.org/x/image/bmp"
)

// RT_ICON 里的 DIB 载荷布局：
//
//	BITMAPINFOHEADER（40 字节，biHeight 通常是真实高度的 2 倍）
//	调色板（bpp <= 8 时存在）
//	XOR 位图（自底向上，每行按 4 字节对齐）
//	AND 掩码（1bpp，自底向上，每行按 4 字节对齐，置位 = 透明）
//
// 除 32bpp 外，调色板与子字节打包的解码全部委托给 golang.org/x/image/bmp：
// 做法是给 XOR 面拼一个 BITMAPFILEHEADER 还原成合法 .bmp 再交给它。
// 32bpp 必须自己来 —— bmp 包对 40 字节头会把 alpha 强制刷成 0xFF
// （其 allowAlpha = infoLen > infoHeaderLen，源码注释里写明不支持 ICO）。

const (
	dibHeaderLen = 40 // BITMAPINFOHEADER
	bmpFileHdr   = 14 // BITMAPFILEHEADER
)

// strideBytes 是一行像素占用的字节数：位数按 32 位边界向上取整。
func strideBytes(w, bpp int) int { return ((w*bpp + 31) / 32) * 4 }

// dibInfo 是从 BITMAPINFOHEADER 里读出来的、后续都要用到的字段。
type dibInfo struct {
	headerLen int
	width     int
	height    int // 头里的原始值，可能是真实高度的 2 倍
	bpp       int
	clrUsed   int
}

func parseDIBHeader(d []byte) (dibInfo, error) {
	var di dibInfo
	if len(d) < dibHeaderLen {
		return di, fmt.Errorf("DIB 头不完整：只有 %d 字节", len(d))
	}
	le := binary.LittleEndian
	di.headerLen = int(le.Uint32(d[0:4]))
	di.width = int(int32(le.Uint32(d[4:8])))
	di.height = int(int32(le.Uint32(d[8:12])))
	di.bpp = int(le.Uint16(d[14:16]))
	di.clrUsed = int(le.Uint32(d[32:36]))

	if di.headerLen != dibHeaderLen {
		return di, fmt.Errorf("不支持的 DIB 头长度 %d（只支持 BITMAPINFOHEADER）", di.headerLen)
	}
	if di.width <= 0 || di.height <= 0 {
		return di, fmt.Errorf("非法的 DIB 尺寸 %dx%d", di.width, di.height)
	}
	if di.width > 1024 || di.height > 2048 {
		return di, fmt.Errorf("DIB 尺寸 %dx%d 超出合理范围", di.width, di.height)
	}
	if comp := le.Uint32(d[16:20]); comp != 0 {
		return di, fmt.Errorf("不支持的 DIB 压缩方式 %d（只支持 BI_RGB）", comp)
	}
	switch di.bpp {
	case 1, 4, 8, 24, 32:
	default:
		return di, fmt.Errorf("不支持的位深 %d", di.bpp)
	}

	// 调色板项数：biClrUsed 为 0 时按 1<<bpp 算（这也是 x/image/bmp 的规则）
	if di.bpp <= 8 {
		if di.clrUsed == 0 {
			di.clrUsed = 1 << di.bpp
		} else if di.clrUsed > 1<<di.bpp {
			return di, fmt.Errorf("调色板项数 %d 超过 %dbpp 的上限", di.clrUsed, di.bpp)
		}
	} else {
		di.clrUsed = 0
	}
	return di, nil
}

// dibSize 只读头部，返回这张 DIB 的真实像素尺寸，不解码像素。
func dibSize(d []byte) (w, h int, err error) {
	di, err := parseDIBHeader(d)
	if err != nil {
		return 0, 0, err
	}
	avail := len(d) - di.headerLen - di.clrUsed*4
	if avail < 0 {
		return 0, 0, fmt.Errorf("DIB 数据被截断：调色板超出载荷")
	}
	imgH, _ := splitPlanes(di.width, di.height, di.bpp, avail)
	if imgH == 0 {
		return 0, 0, fmt.Errorf("DIB 数据被截断：%d 字节装不下 %dx%d@%dbpp", avail, di.width, di.height, di.bpp)
	}
	return di.width, imgH, nil
}

// splitPlanes 判定真实高度以及 AND 掩码是否存在。
//
// biHeight 通常是真实高度的 2 倍（XOR 面 + AND 面），但存在只有 XOR 面的变体，
// 也存在 biHeight 本就是真实高度的变体。不能用 h == 2*w 判断 —— 那会毁掉非正方形图标。
// 四种解释各自蕴含不同的字节总数，取「装得下且剩余最少」的那个即可唯一确定。
func splitPlanes(w, h, bpp, avail int) (imgH int, hasMask bool) {
	xorStride := strideBytes(w, bpp)
	andStride := strideBytes(w, 1)

	type cand struct {
		h    int
		mask bool
	}
	var cands []cand
	if h%2 == 0 {
		cands = append(cands, cand{h / 2, true}, cand{h / 2, false})
	}
	cands = append(cands, cand{h, true}, cand{h, false})

	best, bestSlack := -1, int(^uint(0)>>1)
	for i, c := range cands {
		need := xorStride * c.h
		if c.mask {
			need += andStride * c.h
		}
		if need > avail {
			continue
		}
		if slack := avail - need; slack < bestSlack {
			best, bestSlack = i, slack
		}
	}
	if best < 0 {
		return 0, false
	}
	return cands[best].h, cands[best].mask
}

// decodeDIB 把 RT_ICON 的 DIB 载荷解码成 RGBA。
func decodeDIB(d []byte) (*image.RGBA, error) {
	di, err := parseDIBHeader(d)
	if err != nil {
		return nil, err
	}

	palLen := di.clrUsed * 4
	if di.headerLen+palLen > len(d) {
		return nil, fmt.Errorf("DIB 数据被截断：调色板超出载荷")
	}
	avail := len(d) - di.headerLen - palLen

	imgH, hasMask := splitPlanes(di.width, di.height, di.bpp, avail)
	if imgH == 0 {
		return nil, fmt.Errorf("DIB 数据被截断：%d 字节装不下 %dx%d@%dbpp", avail, di.width, di.height, di.bpp)
	}

	w := di.width
	xorStride := strideBytes(w, di.bpp)
	xorOff := di.headerLen + palLen
	xorLen := xorStride * imgH
	xor := d[xorOff : xorOff+xorLen]

	var and []byte
	if hasMask {
		andStride := strideBytes(w, 1)
		and = d[xorOff+xorLen : xorOff+xorLen+andStride*imgH]
	}

	var img *image.RGBA
	if di.bpp == 32 {
		img = decodeBGRA(xor, w, imgH)
	} else {
		img, err = decodeViaBMP(d, di, imgH, xorLen)
		if err != nil {
			return nil, err
		}
	}

	// 32bpp 有有效 alpha 时以 alpha 为准，忽略 AND 掩码（Windows 的行为）；
	// 老图标 alpha 通道整体为 0，此时才回退用掩码表达透明。
	if and != nil && (di.bpp != 32 || isFullyTransparent(img)) {
		applyMask(img, and, w, imgH)
	}
	return img, nil
}

// decodeBGRA 解码 32bpp 的 XOR 面。DIB 自底向上存储，故 y=0 取最后一行。
func decodeBGRA(xor []byte, w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	stride := w * 4 // 32bpp 天然 4 字节对齐
	for y := 0; y < h; y++ {
		row := xor[(h-1-y)*stride:]
		for x := 0; x < w; x++ {
			o := x * 4
			img.SetRGBA(x, y, color.RGBA{R: row[o+2], G: row[o+1], B: row[o], A: row[o+3]})
		}
	}
	return img
}

// decodeViaBMP 把 XOR 面还原成一个合法的 .bmp 文件再交给 x/image/bmp 解码，
// 借此复用它的调色板、子字节打包和行填充处理。
func decodeViaBMP(d []byte, di dibInfo, imgH, xorLen int) (*image.RGBA, error) {
	palLen := di.clrUsed * 4
	dataOff := bmpFileHdr + di.headerLen + palLen

	buf := make([]byte, 0, dataOff+xorLen)
	hdr := make([]byte, bmpFileHdr)
	le := binary.LittleEndian
	copy(hdr[0:2], "BM")
	le.PutUint32(hdr[2:6], uint32(dataOff+xorLen))
	le.PutUint32(hdr[10:14], uint32(dataOff))
	buf = append(buf, hdr...)

	// 复制原始信息头，把 biHeight 改成真实高度、biSizeImage 改成 XOR 面大小
	info := make([]byte, di.headerLen)
	copy(info, d[:di.headerLen])
	le.PutUint32(info[8:12], uint32(imgH))
	le.PutUint32(info[20:24], uint32(xorLen))
	buf = append(buf, info...)

	// 调色板 + XOR 面
	buf = append(buf, d[di.headerLen:di.headerLen+palLen]...)
	buf = append(buf, d[di.headerLen+palLen:di.headerLen+palLen+xorLen]...)

	src, err := bmp.Decode(bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("解码 %dbpp DIB 失败: %w", di.bpp, err)
	}

	if rgba, ok := src.(*image.RGBA); ok {
		return rgba, nil
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dst.Set(x, y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst, nil
}

// applyMask 套用 AND 掩码：置位表示该像素透明。掩码同样自底向上存储。
func applyMask(img *image.RGBA, and []byte, w, h int) {
	stride := strideBytes(w, 1)
	for y := 0; y < h; y++ {
		row := and[(h-1-y)*stride:]
		for x := 0; x < w; x++ {
			if row[x>>3]>>(7-uint(x&7))&1 == 1 {
				img.SetRGBA(x, y, color.RGBA{})
			}
		}
	}
}

// isFullyTransparent 判断 alpha 通道是否整体为 0（老式 32bpp 图标的特征）。
func isFullyTransparent(img *image.RGBA) bool {
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 0 {
			return false
		}
	}
	return true
}
