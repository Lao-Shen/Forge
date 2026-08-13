# gormx — GORM 工具包

分页查询等常用 GORM 封装。

## 快速开始

```go
import (
    "earwind.top/forge/gormx"
    "gorm.io/gorm"
)

// 基础用法：分页 + 排序
list, total, err := gormx.Paginate[model.Knowledge](
    db.Model(&model.Knowledge{}),
    gormx.NewPage(1, 50),
    "updated_at DESC",
)
fmt.Println(len(list), total) // 当前页数据 + 总条数
```

## 带条件的分页

```go
// 先构造查询条件，再分页
query := db.Model(&model.Knowledge{}).Where("user_id = ?", uid)
if keyword != "" {
    query = query.Where("title LIKE ?", "%"+keyword+"%")
}

list, total, err := gormx.Paginate[model.Knowledge](query, gormx.NewPage(page, 10), "updated_at DESC")
```

## 与 HTTP 请求参数配合

```go
// handler 里解析 query 参数
page, _ := strconv.Atoi(r.URL.Query().Get("page"))
pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

list, total, err := gormx.Paginate[model.Document](
    db.Model(&model.Document{}),
    gormx.NewPage(page, pageSize), // 非法值自动修正（page<1→1，size<1→10）
    "created_at DESC",
)

// 返回给前端
resp.Write(w, resp.Page(list, total))
```

## API 一览

| 类型/函数 | 说明 |
|-----------|------|
| `Page` | 分页参数结构（Page 从 1 开始） |
| `NewPage(page, pageSize)` | 创建分页参数，自动修正非法值 |
| `(*Page).Offset()` | 计算 SQL offset：`(page-1)*pageSize` |
| `Paginate[T](query, page, order)` | 泛型分页：Count 总数 → 查当页数据 |

## 设计说明

- **泛型**：`Paginate[T]` 自动推断结果切片类型，不需要手动 scan
- **order 参数可空**：传 `""` 表示不排序
- **query 传入的是已构建好的 GORM 查询**：所有 Where/Join/Preload 都在调用前加好，本函数只负责 Count + Limit/Offset
- **Count 在 Limit 之前执行**：内部先 Count 再查询，互不干扰
