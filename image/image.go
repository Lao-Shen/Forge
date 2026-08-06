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

// Resize reads an image from src, scales it to fit within maxW×maxH (keeping
// aspect ratio), and writes the result to dstPath.  Supports JPEG and PNG.
func Resize(src io.Reader, maxW, maxH int, dstPath string) error {
	img, format, err := image.Decode(src)
	if err != nil {
		return fmt.Errorf("image decode: %w", err)
	}

	bounds := img.Bounds()
	ow, oh := bounds.Dx(), bounds.Dy()

	// If already within bounds, just copy
	if ow <= maxW && oh <= maxH {
		return saveImage(img, format, dstPath)
	}

	// Scale keeping aspect ratio
	nw, nh := fit(ow, oh, maxW, maxH)
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	return saveImage(dst, "png", dstPath)
}

// fit returns width and height that fit within maxW×maxH preserving aspect ratio.
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

// saveImage encodes and writes img to path in the specified format.
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