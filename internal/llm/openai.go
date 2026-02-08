package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
)

type OpenAIClient struct {
	client openai.Client
	model  string
}

func NewOpenAIClient(apiKey, model, baseURL string) *OpenAIClient {
	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := openai.NewClient(opts...)
	if model == "" {
		model = string(openai.ChatModelGPT4o)
	}
	return &OpenAIClient{client: client, model: model}
}

func (c *OpenAIClient) Chat(ctx context.Context, systemPrompt string, messages []Message, tools []Tool) (*Response, error) {
	// Convert tools
	oaiTools := make([]openai.ChatCompletionToolUnionParam, len(tools))
	for i, t := range tools {
		oaiTools[i] = openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        t.Name,
			Description: openai.String(t.Description),
			Parameters:  openai.FunctionParameters(t.Parameters),
		})
	}

	// Convert messages
	oaiMsgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
	}
	for _, m := range messages {
		switch m.Role {
		case "user":
			if m.ToolCallID != "" {
				oaiMsgs = append(oaiMsgs, openai.ToolMessage(m.Content, m.ToolCallID))
			} else {
				oaiMsgs = append(oaiMsgs, openai.UserMessage(m.Content))
			}
		case "assistant":
			if len(m.ToolCalls) > 0 {
				toolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, len(m.ToolCalls))
				for j, tc := range m.ToolCalls {
					argsJSON, _ := json.Marshal(tc.Params)
					toolCalls[j] = openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID: tc.ID,
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      tc.Name,
								Arguments: string(argsJSON),
							},
						},
					}
				}
				oaiMsgs = append(oaiMsgs, openai.ChatCompletionMessageParamUnion{
					OfAssistant: &openai.ChatCompletionAssistantMessageParam{
						Content: openai.ChatCompletionAssistantMessageParamContentUnion{
							OfString: param.NewOpt(m.Content),
						},
						ToolCalls: toolCalls,
					},
				})
			} else {
				oaiMsgs = append(oaiMsgs, openai.AssistantMessage(m.Content))
			}
		}
	}

	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(c.model),
		Messages: oaiMsgs,
		Tools:    oaiTools,
	})
	if err != nil {
		return nil, fmt.Errorf("openai chat: %w", err)
	}

	if len(resp.Choices) == 0 {
		return &Response{}, nil
	}

	choice := resp.Choices[0]
	result := &Response{
		Content: choice.Message.Content,
	}

	for _, tc := range choice.Message.ToolCalls {
		ftc := tc.AsFunction()
		params := map[string]any{}
		_ = json.Unmarshal([]byte(ftc.Function.Arguments), &params)
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:     ftc.ID,
			Name:   ftc.Function.Name,
			Params: params,
		})
	}

	return result, nil
}
