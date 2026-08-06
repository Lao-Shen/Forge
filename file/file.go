package file

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config 文件上传配置
type Config struct {
	MaxSize   int      // 最大文件大小，单位 MB
	SavePath  string   // 保存目录
	AllowExts []string // 允许的扩展名，如 {".md", ".txt", ".pdf"}
}

// Uploader 文件上传器
type Uploader struct {
	cfg Config
}

// New 创建文件上传器
func New(cfg Config) *Uploader {
	return &Uploader{cfg: cfg}
}

// Save 保存上传文件到磁盘，返回完整存储路径
//
//	src        - 文件内容（通常来自 r.FormFile）
//	size       - 文件字节数
//	originName - 原始文件名（用于校验扩展名）
func (u *Uploader) Save(src io.Reader, size int64, originName string) (string, error) {
	if err := u.validateSize(size); err != nil {
		return "", err
	}

	if err := u.validateExt(originName); err != nil {
		return "", err
	}

	filename := u.uniqueName(originName)
	savePath := filepath.Join(u.cfg.SavePath, filename)

	if err := os.MkdirAll(u.cfg.SavePath, os.ModePerm); err != nil {
		return "", fmt.Errorf("创建上传目录失败: %w", err)
	}

	dst, err := os.Create(savePath)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	return savePath, nil
}

// validateSize 校验文件大小
func (u *Uploader) validateSize(size int64) error {
	if size > int64(u.cfg.MaxSize)*1024*1024 {
		return fmt.Errorf("文件大小超过限制 %dMB", u.cfg.MaxSize)
	}
	return nil
}

// validateExt 校验文件扩展名
func (u *Uploader) validateExt(name string) error {
	ext := strings.ToLower(filepath.Ext(name))
	for _, allowed := range u.cfg.AllowExts {
		if ext == allowed {
			return nil
		}
	}
	return fmt.Errorf("不支持的文件格式: %s", ext)
}

// uniqueName 生成唯一文件名（时间戳 + 原文件名）
func (u *Uploader) uniqueName(origin string) string {
	return UniqueName(origin)
}

// UniqueName 生成唯一文件名（导出版本，供外部使用）
func UniqueName(origin string) string {
	return fmt.Sprintf("%d_%s", time.Now().UnixNano(), origin)
}
