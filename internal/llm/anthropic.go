package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type AnthropicConfig struct {
	BaseURL   string
	AuthToken string
	Model     string
	Timeout   time.Duration
}

type AnthropicClient struct {
	baseURL   string
	authToken string
	model     string
	client    *http.Client
}

func NewAnthropicClient(cfg AnthropicConfig) *AnthropicClient {
	model := cfg.Model
	if model == "" {
		model = "claude-3-5-sonnet-20241022"
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &AnthropicClient{
		baseURL:   strings.TrimRight(cfg.BaseURL, "/"),
		authToken: cfg.AuthToken,
		model:     model,
		client:    &http.Client{Timeout: timeout},
	}
}

func (c *AnthropicClient) Generate(ctx context.Context, req Request) (Response, error) {
	payload := anthropicMessagesRequest{
		Model:     c.model,
		MaxTokens: 512,
		Messages:  make([]anthropicInputMessage, 0, len(req.Messages)+1),
	}

	lastRole := ""
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			payload.System = append(payload.System, anthropicContentBlock{Type: "text", Text: msg.Content})
			continue
		}

		lastRole = msg.Role
		if len(payload.Messages) > 0 && payload.Messages[len(payload.Messages)-1].Role == msg.Role {
			payload.Messages[len(payload.Messages)-1].Content = append(payload.Messages[len(payload.Messages)-1].Content, anthropicContentBlock{
				Type: "text",
				Text: "\n\n" + msg.Content,
			})
			continue
		}

		payload.Messages = append(payload.Messages, anthropicInputMessage{
			Role: msg.Role,
			Content: []anthropicContentBlock{
				{Type: "text", Text: msg.Content},
			},
		})
	}

	if lastRole != "user" {
		nextRound := req.Round + 1
		payload.Messages = append(payload.Messages, anthropicInputMessage{
			Role: "user",
			Content: []anthropicContentBlock{
				{Type: "text", Text: fmt.Sprintf("Continue the agent run and produce round %d according to the existing instructions. Keep the reply concise and preserve any required exact answer tokens from earlier instructions.", nextRound+1)},
			},
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("marshal anthropic request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create anthropic request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("x-api-key", c.authToken)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer httpResp.Body.Close()

	var resp anthropicMessagesResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return Response{}, fmt.Errorf("decode anthropic response: %w", err)
	}

	if httpResp.StatusCode >= 400 {
		return Response{}, fmt.Errorf("%w: status=%d type=%s message=%s", ErrUpstream, httpResp.StatusCode, resp.Error.Type, resp.Error.Message)
	}

	text := ""
	for _, block := range resp.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}

	return Response{
		Message: Message{
			Role:    "assistant",
			Content: strings.TrimSpace(text),
		},
		Done: false,
	}, nil
}

type anthropicMessagesRequest struct {
	Model     string                  `json:"model"`
	MaxTokens int                     `json:"max_tokens"`
	System    []anthropicContentBlock `json:"system,omitempty"`
	Messages  []anthropicInputMessage `json:"messages"`
}

type anthropicInputMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type anthropicMessagesResponse struct {
	Content []anthropicContentBlock `json:"content"`
	Error   struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}
