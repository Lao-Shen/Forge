# pdesc — 项目描述文件协议

每个子项目（app）根目录放一个 `project.json`，用于向 MindVault「我的项目」模块描述该项目。扫描描述文件即可自动收集资料、更新日志等素材。

## 为什么用 JSON 而不是 YAML

- Go 标准库直接解析，**零依赖**
- TypeScript 前端 `JSON.stringify` 直接生成
- 协议字段扁平，不需要 YAML 的注释等高级特性

## 文件格式

文件名固定：`project.json`

```json
{
  "name": "MindVault Web",
  "docs": ["README.md", "docs/部署.md"],
  "repository": "https://github.com/earwind/MindVault",
  "changelog": "CHANGELOG.md",
  "build": ["docker compose up -d --build"]
}
```

### 字段说明

| 字段 | 必填 | 类型 | 说明 |
|------|------|------|------|
| `name` | ✅ | string | 项目名称（展示用） |
| `docs` | ❌ | string[] | 文档相对路径数组（相对 project.json 所在目录），用于收集打包文档 |
| `repository` | ❌ | string | 代码仓库地址 |
| `changelog` | ❌ | string | 更新日志路径，缺省默认 `changelog.json` |
| `build` | ❌ | string[] | 构建命令数组（按顺序执行） |

所有相对路径都**相对于 project.json 所在目录**。

## 更新日志格式（changelog.json）

日期为 key，条目为数组：

```json
{
  "2026-08-16": ["新增 xxx 功能", "修复 yyy 问题"],
  "2026-08-14": ["项目上线"]
}
```

## 快速开始

```go
import "earwind.top/forge/pdesc"

// 1. 加载描述文件
p, err := pdesc.Load("/path/to/app")          // 读 /path/to/app/project.json
// 或指定文件
p, err := pdesc.LoadFile("/path/to/project.json")

// 2. 读项目资料（按 docs 路径收集文档内容）
base := filepath.Dir("/path/to/project.json")
docs, err := p.LoadDocs(base)                  // []Doc{Path, Content}

// 3. 读更新日志（JSON，日期为 key）
cl, err := p.LoadChangelog(base)               // Changelog，不存在返回空
for _, date := range cl.SortedDates() {        // 日期倒序
    for _, item := range cl[date] {
        fmt.Println(date, item)
    }
}

// 4. 记录一条更新并保存
cl.Add("2026-08-16", "新增 xxx")
p.SaveChangelog(base, cl)

// 5. 保存（生成/覆盖描述文件）
err = pdesc.Save("/path/to/app", p)
```

## API 一览

| 函数/方法 | 说明 |
|-----------|------|
| `Load(dir)` | 从目录读 `project.json` |
| `LoadFile(path)` | 从指定文件读 |
| `Save(dir, p)` | 保存到目录（缩进格式化） |
| `(*Project).LoadDocs(baseDir)` | 按 docs 路径读全部文档内容 |
| `(*Project).LoadChangelog(baseDir)` | 读更新日志 JSON（缺省 changelog.json） |
| `(*Project).SaveChangelog(baseDir, c)` | 保存更新日志 JSON |
| `(Changelog).Add(date, item)` | 给指定日期追加一条记录 |
| `(Changelog).SortedDates()` | 日期倒序列表 |

## 使用场景

1. **桌面端同步**：扫描本地项目目录 → `pdesc.Load` → 上传 docs + changelog 到服务端
2. **Web 展示**：服务端存储的资料由 pdesc 协议收集而来，字段一一对应
3. **CI/发布**：`build` 命令数组可供发布脚本读取执行
