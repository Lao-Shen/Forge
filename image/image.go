// Package image provides basic image processing utilities (resize/crop).
// Uses only Go standard library + golang.org/x/image/draw for high-quality scaling.
package image

import (
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
)

// ResizeSquare center-crops the source to a square, then scales to exactly size×size.
// The result is always a square PNG suitable for avatars.
func ResizeSquare(src io.Reader, size int, dstPath string) error {
	img, _, err := image.Decode(src)
	if err != nil {
		return fmt.Errorf("image decode: %w", err)
	}

	bounds := img.Bounds()
	ow, oh := bounds.Dx(), bounds.Dy()

	// Center-crop to square
	var crop image.Image
	if ow > oh {
		// landscape: crop left/right
		x0 := (ow - oh) / 2
		crop = img.(interface {
			SubImage(r image.Rectangle) image.Image
		}).SubImage(image.Rect(x0, 0, x0+oh, oh))
	} else if oh > ow {
		// portrait: crop top/bottom
		y0 := (oh - ow) / 2
		crop = img.(interface {
			SubImage(r image.Rectangle) image.Image
		}).SubImage(image.Rect(0, y0, ow, y0+ow))
	} else {
		crop = img
	}

	// Scale to exact target size
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(dst, dst.Bounds(), crop, crop.Bounds(), draw.Over, nil)

	return savePNG(dst, dstPath)
}

// savePNG encodes and writes img to path as PNG.
func savePNG(img image.Image, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	return png.Encode(f, img)
}

// Resize reads an image from src, scales it to fit within maxW×maxH (keeping
// aspect ratio), and writes the result to dstPath.  Supports JPEG and PNG.
func Resize(src io.Reader, maxW, maxH int, dstPath string) error {
	img, format, err := image.Decode(src)
	if err != nil {
		return fmt.Errorf("image decode: %w", err)
	}

	bounds := img.Bounds()
	ow, oh := bounds.Dx(), bounds.Dy()

	nw, nh := fit(ow, oh, maxW, maxH)
	if nw == ow && nh == oh {
		return saveImage(img, format, dstPath)
	}

	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	return saveImage(dst, "png", dstPath)
}

func fit(ow, oh, maxW, maxH int) (int, int) {
	wRatio := float64(maxW) / float64(ow)
	hRatio := float64(maxH) / float64(oh)
	ratio := wRatio
	if hRatio < wRatio {
		ratio = hRatio
	}
	if ratio >= 1 {
		return ow, oh
	}
	return int(float64(ow) * ratio), int(float64(oh) * ratio)
}

func saveImage(img image.Image, format, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		return jpeg.Encode(f, img, &jpeg.Options{Quality: 85})
	default:
		return png.Encode(f, img)
	}
}

// IsImageExt reports whether ext (including dot) is a supported image format.
func IsImageExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png":
		return true
	}
	return false
}

// ValidateImage reads a small prefix to check it's a valid image.
func ValidateImage(r io.Reader) error {
	_, format, err := image.DecodeConfig(r)
	if err != nil {
		return errors.New("无法识别图片格式")
	}
	if format != "jpeg" && format != "png" {
		return fmt.Errorf("不支持的图片格式: %s", format)
	}
	return nil
}
