# Forge

技术沉淀库 — 常用 Go 函数封装，带文档，方便回顾和跨项目复用。

> 命名寓意：Forge（铁匠铺）= 锤炼常用工具的地方。

## 包索引

| 包 | 用途 | 何时用 |
|----|------|--------|
| [`jwt`](jwt/) | JWT token 生成/解析 | 任何需要登录鉴权的 Web 项目 |
| [`resp`](resp/) | 统一 HTTP 响应格式 | 任何提供 JSON API 的 Go 服务 |
| [`file`](file/) | 文件上传校验 + 落盘 | 需要处理文件上传时 |
| [`ai`](ai/) | AI 服务调用统一接口 | 需要接 OpenAI / Claude 时 |

## 版本策略

不追求发版 — `main` 分支随时可用。项目中通过 `go.mod` 的 `replace` 指令指向本地：

```go
// 开发阶段
replace earwind.top/forge => ../Forge

// 发布后（推到 GitHub 后）
// replace earwind.top/forge => github.com/earwind/forge v0.1.0
```

## 添加新包规则

1. 每个包一个文件夹
2. 每个文件夹必须包含 `README.md`（用法 + 示例）
3. 优先用结构体封装，避免包级全局变量
4. 依赖尽量少，一个包只做一件事
