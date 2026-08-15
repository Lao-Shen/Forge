// Package pdesc 定义「项目描述文件」协议。
//
// 每个子项目（app）根目录放一个 project.json，描述该项目的基本信息：
// 名称、文档路径、代码仓库、更新日志路径、构建命令。
// 这套协议服务于 MindVault 的「我的项目」模块：扫描各项目目录，
// 自动收集资料（docs）、更新日志（changelog）等素材上传展示。
//
// 文件格式：JSON（标准库零依赖，Go/TS 均可直接序列化）
package pdesc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FileName 描述文件的固定文件名
const FileName = "project.json"

// Project 项目描述（project.json 的完整结构）
//
// 示例：
//
//	{
//	  "name": "MindVault Web",
//	  "docs": ["README.md", "docs/部署.md"],
//	  "repository": "https://github.com/earwind/MindVault",
//	  "changelog": "CHANGELOG.md",
//	  "build": ["docker compose up -d --build"]
//	}
type Project struct {
	// Name 项目名称（用于展示）
	Name string `json:"name"`

	// Docs 文档相对路径数组（相对于描述文件所在目录）
	// 用于把文档收集起来统一打包/上传，避免展示端找不到文件
	Docs []string `json:"docs"`

	// Repository 代码仓库地址（可选，没有则留空）
	Repository string `json:"repository,omitempty"`

	// Changelog 更新日志文件相对路径（可选，默认 changelog.md）
	Changelog string `json:"changelog,omitempty"`

	// Build 构建命令数组（可选，按顺序执行）
	Build []string `json:"build,omitempty"`
}

// Load 从目录加载描述文件（dir/project.json），文件不存在返回错误
func Load(dir string) (*Project, error) {
	return LoadFile(filepath.Join(dir, FileName))
}

// LoadFile 从指定路径加载描述文件
func LoadFile(path string) (*Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取描述文件失败: %w", err)
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("描述文件格式错误: %w", err)
	}
	if p.Name == "" {
		return nil, fmt.Errorf("描述文件缺少必填字段 name")
	}
	return &p, nil
}

// Save 保存描述文件到目录（dir/project.json）
func Save(dir string, p *Project) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, FileName), data, 0644)
}

// BaseDir 返回描述文件所在目录（加载路径的目录部分）
func (p *Project) BaseDir(path string) string {
	return filepath.Dir(path)
}

// Doc 文档（名称 + 内容）
type Doc struct {
	Path    string // 相对路径
	Content []byte // 文件内容
}

// LoadDocs 按 Docs 相对路径读取所有文档内容
//
//	baseDir - 描述文件所在目录
func (p *Project) LoadDocs(baseDir string) ([]Doc, error) {
	docs := make([]Doc, 0, len(p.Docs))
	for _, rel := range p.Docs {
		path := filepath.Join(baseDir, filepath.FromSlash(rel))
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("读取文档 %s 失败: %w", rel, err)
		}
		docs = append(docs, Doc{Path: rel, Content: content})
	}
	return docs, nil
}

