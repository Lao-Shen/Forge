package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

// DeepSeekConfig DeepSeek 配置
type DeepSeekConfig struct {
	APIKey string // 必填，sk- 开头
	Model  string // 默认 deepseek-v4-flash（可选 deepseek-v4-pro）
	// BaseURL 可选，默认 https://api.deepseek.com
	// 覆盖场景：代理服务 / 兼容其他 OpenAI 格式的服务
	BaseURL string
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
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		baseURL: cfg.BaseURL,
		http:    &http.Client{Timeout: 60 * time.Second},
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

// ========== OpenAI 兼容格式 ==========

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
