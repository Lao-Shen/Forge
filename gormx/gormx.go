// Package gormx provides GORM helpers: pagination and common query utilities.
package gormx

import (
	"gorm.io/gorm"
)

// Page 分页参数（页码从 1 开始）
type Page struct {
	Page     int `json:"page"`      // 页码，1 起
	PageSize int `json:"page_size"` // 每页条数
}

// NewPage 创建分页参数，自动修正非法值
//
//	page < 1 → 1
//	pageSize < 1 → 10
func NewPage(page, pageSize int) *Page {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	return &Page{Page: page, PageSize: pageSize}
}

// Offset 计算 SQL offset
func (p *Page) Offset() int {
	return (p.Page - 1) * p.PageSize
}

// Paginate 执行分页查询：先 Count 总数，再查当页数据
//
//	query - 已构造好的 GORM 查询（含 Model、Where 等条件）
//	p     - 分页参数
//	order - 排序语句，如 "updated_at DESC"，为空则不排序
//
// 返回：当页数据、总条数、错误
//
//	示例：
//	list, total, err := gormx.Paginate[model.Knowledge](
//	    db.Model(&model.Knowledge{}).Where("user_id = ?", uid),
//	    gormx.NewPage(1, 50),
//	    "updated_at DESC",
//	)
func Paginate[T any](query *gorm.DB, p *Page, order string) ([]T, int64, error) {
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []T
	q := query
	if order != "" {
		q = q.Order(order)
	}
	if err := q.Offset(p.Offset()).Limit(p.PageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}
