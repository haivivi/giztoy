package minimax_interface

import (
	"context"
	"iter"
)

// TextService 文本生成服务接口
type TextService interface {
	// CreateChatCompletion 创建聊天补全（兼容 OpenAI 风格）
	CreateChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error)

	// CreateChatCompletionStream 创建流式聊天补全
	//
	// 返回一个迭代器，可以直接 for range 遍历。
	// 迭代结束或 break 时会自动关闭连接。
	//
	// 示例:
	//
	//	for chunk, err := range client.Text.CreateChatCompletionStream(ctx, req) {
	//	    if err != nil {
	//	        return err
	//	    }
	//	    fmt.Print(chunk.Delta.Content)
	//	}
	CreateChatCompletionStream(ctx context.Context, req *ChatCompletionRequest) iter.Seq2[*ChatCompletionChunk, error]
}

// ChatCompletionRequest 聊天补全请求
type ChatCompletionRequest struct {
	// Model 模型名称
	Model string `json:"model"`

	// Messages 消息列表
	Messages []Message `json:"messages"`

	// MaxTokens 最大输出 token 数
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature 采样温度，范围 0-2
	Temperature float64 `json:"temperature,omitempty"`

	// TopP 核采样参数
	TopP float64 `json:"top_p,omitempty"`

	// Tools 工具定义列表
	Tools []Tool `json:"tools,omitempty"`

	// ToolChoice 工具选择策略
	ToolChoice any `json:"tool_choice,omitempty"`
}

// Message 消息
type Message struct {
	// Role 角色: system, user, assistant
	Role string `json:"role"`

	// Content 消息内容（文本或内容数组）
	Content any `json:"content"`

	// ToolCalls 工具调用（assistant 消息）
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// ToolCallID 工具调用 ID（tool 消息）
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// Tool 工具定义
type Tool struct {
	Type     string       `json:"type"`
	Function FunctionTool `json:"function"`
}

// FunctionTool 函数工具定义
type FunctionTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function FunctionToolCall `json:"function"`
}

// FunctionToolCall 函数调用
type FunctionToolCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatCompletionResponse 聊天补全响应
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage"`
}

// Choice 选择
type Choice struct {
	Index        int      `json:"index"`
	Message      *Message `json:"message"`
	FinishReason string   `json:"finish_reason"`
}

// Usage 使用量
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionChunk 流式聊天补全块
type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
}

// ChunkChoice 流式选择
type ChunkChoice struct {
	Index        int          `json:"index"`
	Delta        *ChunkDelta  `json:"delta"`
	FinishReason string       `json:"finish_reason,omitempty"`
}

// ChunkDelta 流式增量内容
type ChunkDelta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}
