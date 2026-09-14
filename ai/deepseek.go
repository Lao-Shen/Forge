package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// DeepSeekClient DeepSeek 大模型客户端
//
// API 与 OpenAI 完全兼容：
//
//	POST https://api.deepseek.com/chat/completions
//
// 文档：https://api-docs.deepseek.com/zh-cn/
type DeepSeekClient struct {
	apiKey       string
	model        string
	baseURL      string
	thinkingMode string // "" | "disabled" | "enabled"
	http         *http.Client
}

// DeepSeekConfig DeepSeek 配置
type DeepSeekConfig struct {
	APIKey string // 必填，sk- 开头
	Model  string // 默认 deepseek-v4-flash（可选 deepseek-v4-pro）
	// BaseURL 可选，默认 https://api.deepseek.com
	// 覆盖场景：代理服务 / 兼容其他 OpenAI 格式的服务
	BaseURL string
	// ThinkingMode 思考模式（V4 服务端默认开启，思考 token 单独计费）：
	//   ""         不干预，跟随服务默认
	//   "disabled" 显式关闭思考（省 token）
	//   "enabled"  显式开启思考（响应含 reasoning_content，可经 StreamChatWithThinking 取到）
	ThinkingMode string
}

// NewDeepSeek 创建 DeepSeek 客户端
func NewDeepSeek(cfg DeepSeekConfig) *DeepSeekClient {
	if cfg.Model == "" {
		cfg.Model = "deepseek-v4-flash"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.deepseek.com"
	}
	return &DeepSeekClient{
		apiKey:       cfg.APIKey,
		model:        cfg.Model,
		baseURL:      cfg.BaseURL,
		thinkingMode: cfg.ThinkingMode,
		// 120s 总超时（DeepSeek 高峰期首 token 延迟大）；连接/TLS 阶段 10s 快速失败，
		// 网络不通（防火墙 DROP 等）时 10 秒内报错，而不是挂满总超时
		http: &http.Client{
			Timeout: 120 * time.Second,
			Transport: &http.Transport{
				DialContext:         (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
	}
}

// Chat 发送对话，返回 AI 回复文本
//
// 实现 ai.Client 接口，可直接用于 Summarize 等封装。
func (c *DeepSeekClient) Chat(ctx context.Context, messages []Message) (string, error) {
	reqBody := chatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
		Thinking: c.thinking(),
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("DeepSeek 返回 %d: %s", resp.StatusCode, string(body))
	}

	var result chatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("DeepSeek 返回空响应")
	}

	return result.Choices[0].Message.Content, nil
}

// StreamChat 流式对话：逐段回调 onDelta 增量文本（OpenAI 兼容 SSE 格式）
//
// 不设总超时（由 ctx 控制生命周期），长回复不会中途断流。
// 实现 ai.StreamClient 接口。
func (c *DeepSeekClient) StreamChat(ctx context.Context, messages []Message, onDelta func(delta string) error) error {
	return c.streamChat(ctx, messages, nil, onDelta)
}

// StreamChatWithThinking 流式对话：分别回调思考内容（reasoning_content）与正文增量
//
//	思考模式（ThinkingMode: "enabled"）下 onThinking 逐段收到推理内容；
//	非思考模式下 onThinking 不会收到任何数据。onThinking 可传 nil。
func (c *DeepSeekClient) StreamChatWithThinking(ctx context.Context, messages []Message, onThinking func(delta string) error, onDelta func(delta string) error) error {
	return c.streamChat(ctx, messages, onThinking, onDelta)
}

// streamChat 流式实现：解析 SSE，分别投递 reasoning_content 与 content
func (c *DeepSeekClient) streamChat(ctx context.Context, messages []Message, onThinking func(delta string) error, onDelta func(delta string) error) error {
	reqBody := chatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   true,
		Thinking: c.thinking(),
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	// 流式请求生命周期由 ctx 控制，不复用带 60s 超时的默认 client
	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DeepSeek 返回 %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024), 1024*1024) // 单行可能很长
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "data: [DONE]" {
			break
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		payload := strings.TrimPrefix(line, "data: ")
		var chunk streamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		if delta.ReasoningContent != "" && onThinking != nil {
			if err := onThinking(delta.ReasoningContent); err != nil {
				return err
			}
		}
		if delta.Content != "" {
			if err := onDelta(delta.Content); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取流失败: %w", err)
	}
	return nil
}

// ========== OpenAI 兼容格式 ==========

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	// Thinking 思考模式开关（V4）：{"type":"enabled"|"disabled"}，nil 不干预
	Thinking *thinkingParam `json:"thinking,omitempty"`
}

type thinkingParam struct {
	Type string `json:"type"`
}

// thinking 按 ThinkingMode 生成请求参数（"" 返回 nil 不干预）
func (c *DeepSeekClient) thinking() *thinkingParam {
	if c.thinkingMode == "" {
		return nil
	}
	return &thinkingParam{Type: c.thinkingMode}
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// streamChunk SSE 流式响应片段（OpenAI 兼容格式）
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"` // 思考模式下的推理内容
		} `json:"delta"`
	} `json:"choices"`
}

// WebSearch 服务端联网搜索问答（Responses API + web_search 工具）
//
// 模型自主决定搜索词、打开网页并基于结果回答（搜索在 DeepSeek 服务端执行，无需第三方搜索 key）。
// 注意：目前仅 deepseek-v4-flash 支持 web_search（chat/completions 端点会报 400），
// 创建客户端时请显式传 Model: "deepseek-v4-flash"。
//
// 返回 answer 最终回答；sources 为模型打开过的参考页面 URL（open_page 记录，已去重）。
func (c *DeepSeekClient) WebSearch(ctx context.Context, question string) (answer string, sources []string, err error) {
	reqBody := map[string]interface{}{
		"model": c.model,
		"input": question,
		"tools": []map[string]string{{"type": "web_search"}},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/responses", bytes.NewReader(data))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// 联网搜索耗时较长（10~30 秒），用独立的 120s 超时
	searchClient := &http.Client{Timeout: 120 * time.Second}
	resp, err := searchClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("DeepSeek 返回 %d: %s", resp.StatusCode, string(body))
	}

	var result responsesResp
	if err := json.Unmarshal(body, &result); err != nil {
		return "", nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 遍历 output：收集 open_page 的 URL 作为来源；拼接 message 的 output_text 作为回答
	var sb strings.Builder
	seen := map[string]bool{}
	for _, item := range result.Output {
		switch item.Type {
		case "web_search_call":
			if item.Action.Type == "open_page" && item.Action.URL != "" {
				url := cleanSourceURL(item.Action.URL)
				if !seen[url] {
					seen[url] = true
					sources = append(sources, url)
				}
			}
		case "message":
			for _, part := range item.Content {
				if part.Type == "output_text" {
					sb.WriteString(part.Text)
				}
			}
		}
	}

	if sb.Len() == 0 {
		return "", sources, fmt.Errorf("DeepSeek 返回空响应")
	}
	return sb.String(), sources, nil
}

// responsesResp Responses API 响应（仅解析所需字段）
type responsesResp struct {
	Output []struct {
		Type   string `json:"type"`
		Action struct {
			Type    string   `json:"type"`
			Queries []string `json:"queries"`
			URL     string   `json:"url"`
		} `json:"action"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

// cleanSourceURL 去掉 open_page URL 末尾的 #ws_call_id=... 跟踪尾巴
func cleanSourceURL(url string) string {
	if i := strings.Index(url, "#ws_call_id="); i >= 0 {
		return url[:i]
	}
	return url
}
