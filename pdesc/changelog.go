package pdesc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// DefaultChangelogName 默认更新日志文件名
const DefaultChangelogName = "changelog.json"

// Changelog 更新日志（日期为 key，条目为数组）
//
//	{
//	  "2026-08-16": ["新增 xxx", "修复 yyy"],
//	  "2026-08-14": ["项目上线"]
//	}
type Changelog map[string][]string

// LoadChangelog 读取项目的更新日志（JSON 格式）
//
//	路径取自 Project.Changelog，缺省 changelog.json
//	文件不存在返回空 Changelog（不报错）
func (p *Project) LoadChangelog(baseDir string) (Changelog, error) {
	rel := p.Changelog
	if rel == "" {
		rel = DefaultChangelogName
	}
	data, err := os.ReadFile(filepath.Join(baseDir, filepath.FromSlash(rel)))
	if err != nil {
		if os.IsNotExist(err) {
			return Changelog{}, nil
		}
		return nil, fmt.Errorf("读取更新日志失败: %w", err)
	}

	var c Changelog
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("更新日志格式错误: %w", err)
	}
	return c, nil
}

// SaveChangelog 保存更新日志到文件（JSON 缩进格式）
func (p *Project) SaveChangelog(baseDir string, c Changelog) error {
	rel := p.Changelog
	if rel == "" {
		rel = DefaultChangelogName
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(baseDir, filepath.FromSlash(rel)), data, 0644)
}

// Add 给指定日期追加一条更新记录（同日多条自动追加）
func (c Changelog) Add(date, item string) {
	if _, ok := c[date]; !ok {
		c[date] = []string{}
	}
	c[date] = append(c[date], item)
}

// SortedDates 返回按日期倒序（最新在前）的日期列表
func (c Changelog) SortedDates() []string {
	dates := make([]string, 0, len(c))
	for d := range c {
		dates = append(dates, d)
	}
	// 日期格式 YYYY-MM-DD 可直接字典序倒排
	sort.Slice(dates, func(i, j int) bool { return dates[i] > dates[j] })
	return dates
}
