# jwt

JWT token 生成和解析的轻量封装。

## 依赖

- `github.com/golang-jwt/jwt/v5`

## 使用

```go
import "earwind.top/forge/jwt"

func main() {
    // 1. 创建 JWT 实例（启动时初始化一次）
    j := jwt.New("your-secret-key", 7200) // 2 小时过期

    // 2. 生成 token
    token, err := j.Generate(1, "admin")
    // token = "eyJhbGciOiJIUzI1NiIs..."

    // 3. 解析 token
    claims, err := j.Parse(token)
    // claims.UserID   = 1
    // claims.Username = "admin"
}
```

## 扩展自定义 Claims

如果你的业务需要额外的 JWT 字段（如角色、权限），直接在内嵌 `jwt.Claims` 的结构体上扩展即可，不需要改本库代码。

## 为什么不用全局变量？

旧版本用 `InitJWT()` + 包级变量，缺点是：
- 无法在测试中同时创建不同密钥的实例
- 全局状态隐式依赖，排查问题困难

现在的结构体方式：一个 `jwt.New()` 就是一个独立的 JWT 实例，干净、可测试。
