package ai

import "context"

// Message 聊天消息
type Message struct {
	Role    string `json:"role"`    // "system" | "user" | "assistant"
	Content string `json:"content"`
}

// Client AI 客户端接口
//
// 不同的 AI 服务（OpenAI / Claude / 本地模型）实现这个接口即可
type Client interface {
	Chat(ctx context.Context, messages []Message) (string, error)
}

// StreamClient 支持流式输出的 AI 客户端接口
//
// 聊天场景优先使用它：逐段回调增量文本，前端可做打字机效果。
type StreamClient interface {
	Client
	// StreamChat 流式对话，每收到一段增量文本回调一次 onDelta
	StreamChat(ctx context.Context, messages []Message, onDelta func(delta string) error) error
}

// ========== 一个最简单的调用封装 ==========

// Summarize 调用 AI 对文本做摘要 + 提取标签
//
// 这是一个上层封装，依赖 Client 接口。不需要具体知道底层是 OpenAI 还是 Claude。
func Summarize(ctx context.Context, c Client, content string) (*SummarizeResult, error) {
	prompt := buildPrompt(content)

	reply, err := c.Chat(ctx, []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, err
	}

	return parseResult(reply), nil
}

// SummarizeResult 总结结果
type SummarizeResult struct {
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
}

// ========== 内部实现（不导出） ==========

const systemPrompt = `你是一个知识整理助手。请对用户提供的文档内容生成 200 字以内的中文摘要，并提取 3-5 个关键词标签。`

func buildPrompt(content string) string {
	return content
}

func parseResult(reply string) *SummarizeResult {
	// TODO: 根据实际 AI 返回的 JSON 格式解析
	return &SummarizeResult{
		Summary: reply,
		Tags:    []string{},
	}
}
