# ai

AI 服务调用的统一接口定义 + 具体实现。

## 设计思路

Go 没有"类"，但有 `interface` — 它允许多个不同实现满足同一个接口。本包定义 `Client` 接口，具体实现（DeepSeek / OpenAI / Claude / 本地 Ollama）都满足 `ai.Client`；支持流式的客户端额外实现 `ai.StreamClient`。

```
┌──────────────────────────────────┐
│  ai.Client 接口                   │
│  Chat(ctx, []Message) string      │
│  ├─ ai.StreamClient（流式扩展）    │
│  │  StreamChat(ctx, msgs, onDelta)│
└──────────┬───────────────────────┘
           │
   ┌───────┼───────┬──────────┐
   ▼       ▼       ▼          ▼
DeepSeek OpenAI  Claude    未来扩展...
```

## 使用 DeepSeek（已实现）

```go
import (
    "context"
    "earwind.top/forge/ai"
)

func main() {
    // 1. 创建客户端（API Key 在 https://platform.deepseek.com 申请）
    client := ai.NewDeepSeek(ai.DeepSeekConfig{
        APIKey: "sk-xxx",
        Model:  "deepseek-chat",
    })

    // 2. 直接对话
    reply, err := client.Chat(context.Background(), []ai.Message{
        {Role: "system", Content: "你是知识管理助手"},
        {Role: "user", Content: "帮我总结 Go 的并发模型"},
    })
    fmt.Println(reply)

    // 3. 用于文档总结
    result, _ := ai.Summarize(context.Background(), client, docContent)
    fmt.Println(result.Summary, result.Tags)

    // 4. 流式对话（打字机效果）
    var sb strings.Builder
    _ = client.StreamChat(context.Background(), []ai.Message{
        {Role: "user", Content: "用三句话介绍 Go"},
    }, func(delta string) error {
        sb.WriteString(delta)
        fmt.Print(delta) // 逐段输出增量文本
        return nil
    })
}
```

### 配置项

| 字段 | 必填 | 说明 |
|------|------|------|
| APIKey | ✅ | `sk-` 开头的密钥 |
| Model | ❌ | 默认 `deepseek-v4-flash`（可选 `deepseek-v4-pro`） |
| BaseURL | ❌ | 默认 `https://api.deepseek.com`，代理场景可覆盖 |

### 特点

- 原生 `net/http` 实现，零额外依赖
- 普通对话超时 60 秒；流式对话不设总超时，由 `ctx` 控制生命周期（长回复不断流）
- 非 200 响应会把错误体原文返回，方便排查

## 上层封装

`ai.Summarize(ctx, client, content)` — 对文档做摘要 + 提取标签，任何实现 `Client` 的客户端都能用。

## 当前状态

- [x] `Client` 接口定义
- [x] `Summarize` 上层封装
- [x] DeepSeek 实现
- [x] 流式输出（`StreamClient` / `StreamChat`，SSE 解析）
- [ ] OpenAI 实现（结构相同，改 BaseURL 即可）
- [ ] Claude 实现
