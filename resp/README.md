# resp

Go Web 项目的统一 HTTP 响应格式。

每个接口返回的 JSON 结构一致，前端只需一套判断逻辑：

```json
// 成功（Code == 0）
{ "code": 0, "message": "ok", "data": {...} }

// 失败（Code != 0）
{ "code": 3, "message": "参数错误" }
```

## 使用

```go
import "earwind.top/forge/resp"

// 返回成功（不带数据）
resp.Write(w, resp.OK())

// 返回成功（带数据）
resp.Write(w, resp.Data(user))

// 返回分页数据
resp.Write(w, resp.Page(list, total))

// 返回通用错误
resp.Write(w, resp.Error("用户名已存在"))

// 返回指定错误码
resp.Write(w, resp.ErrCode(resp.CodeAuthErr, ""))
```

## 错误码

| 常量 | 值 | 含义 |
|------|-----|------|
| `CodeOK` | 0 | 成功 |
| `CodeErr` | 1 | 通用错误 |
| `CodeAuthErr` | 2 | 未登录或 token 过期 |
| `CodeParamErr` | 3 | 参数校验失败 |
| `CodeNotFound` | 4 | 资源不存在 |

项目特有的错误码（如"余额不足"、"权限不够"）在业务代码里追加常量即可，不用改本库。
