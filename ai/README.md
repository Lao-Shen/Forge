# ai

AI 服务调用的统一接口定义。

## 设计思路

Go 没有"类"，但有 `interface` — 它允许多个不同实现满足同一个接口。本包定义 `Client` 接口，具体实现（OpenAI / Claude / 本地 Ollama）各自写自己的包，都满足 `ai.Client`。

```
┌─────────────────────────────┐
│  ai.Client 接口              │
│  Chat(ctx, []Message) string │
└──────────┬──────────────────┘
           │
   ┌───────┼───────┬──────────┐
   ▼       ▼       ▼          ▼
 OpenAI  Claude  Ollama   未来扩展...
```

## 使用

```go
import (
    "earwind.top/forge/ai"
    // 具体的 AI 实现包（待开发）
    // openai "github.com/earwind/..."
)

func main() {
    var client ai.Client
    client = openai.New("sk-xxx", "gpt-4o")

    result, _ := ai.Summarize(context.Background(), client, docContent)
    // result.Summary = "本文介绍了..."
    // result.Tags    = ["Go", "并发"]
}
```

## 当前状态

- [x] `Client` 接口定义
- [x] `Summarize` 上层封装
- [ ] OpenAI 实现
- [ ] Claude 实现
