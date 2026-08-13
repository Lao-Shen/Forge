# auth — 用户认证包

注册 / 登录 / 修改密码 / 头像更新，一套方法全搞定。

## 特点

- 只依赖 `*gorm.DB` + 表名，**不绑定具体 User 模型**——项目的 User 表只要有标准列就能用
- 密码 bcrypt 加密存储，原始密码绝不落库
- JWT 签发复用 [`forge/jwt`](../jwt/)
- 错误消息中文，可直接透传给前端

## 要求

用户表需要以下列（GORM 迁移自动创建的标准结构即可）：

| 列 | 类型 | 说明 |
|----|------|------|
| id | uint | 主键 |
| username | string | 唯一索引 |
| password | string | bcrypt 哈希 |
| avatar | string | 头像路径（可空） |
| created_at | datetime | 创建时间 |

## 快速开始

```go
import (
    "earwind.top/forge/auth"
    "earwind.top/forge/jwt"
    "gorm.io/driver/mysql"
    "gorm.io/gorm"
)

func main() {
    db, _ := gorm.Open(mysql.Open("..."), &gorm.Config{})

    // 1. 自动建表（表结构用 auth.User 作为模板）
    db.AutoMigrate(&auth.User{})

    // 2. 创建认证服务
    j := jwt.New("your-secret", 7200) // 2 小时过期
    svc := auth.New(db, j, "users")

    // 3. 使用
    u, err := svc.Register("alice", "secret123")  // 注册
    token, err := svc.Login("alice", "secret123") // 登录 → JWT token
    user, err := svc.GetUser(u.ID)                // 查用户信息
    err = svc.ChangePassword(u.ID, "secret123", "newpass456") // 改密
    err = svc.UpdateAvatar(u.ID, "/uploads/avatars/a.png")    // 更新头像
}
```

## 配合已有项目（模型已存在）

项目已有自己的 User 模型也没关系，只要表名一致、列名一致：

```go
// 项目的 User 模型（比 auth.User 多字段）
type User struct {
    ID       uint   `gorm:"primaryKey"`
    Username string `gorm:"size:50;uniqueIndex;not null"`
    Password string `gorm:"size:255;not null"`
    Avatar   string
    Company  string // 项目特有字段，auth 包不关心
}

// 直接用表名接入
svc := auth.New(db, j, "users")
token, _ := svc.Login("alice", "secret123")
```

## API 一览

| 方法 | 签名 | 说明 |
|------|------|------|
| `New` | `(db, jwt, table) *Service` | 创建服务 |
| `Register` | `(username, password) (*User, error)` | 注册（校验+加密+入库） |
| `Login` | `(username, password) (string, error)` | 登录，返回 JWT |
| `GetUser` | `(userID) (*User, error)` | 查询用户（密码字段不序列化） |
| `ChangePassword` | `(userID, old, new) error` | 改密（验证旧密码） |
| `UpdateAvatar` | `(userID, path) error` | 更新头像路径 |

## 校验规则

- 用户名：3~50 字符，唯一
- 密码：至少 6 字符
- 登录/改密密码错误统一返回「用户名或密码错误」/「旧密码错误」
