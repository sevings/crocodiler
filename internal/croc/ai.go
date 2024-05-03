package croc

import (
	"context"
	"fmt"
	"github.com/erni27/imcache"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/mistral"
	"github.com/tmc/langchaingo/llms/openai"
	"time"
)

type aiChat struct {
	messages []llms.MessageContent
	maxHst   int
}

func newAiChat(prompt string, maxHistory int) *aiChat {
	ai := &aiChat{
		messages: make([]llms.MessageContent, 0, 3),
		maxHst:   maxHistory,
	}

	ai.addMessage(llms.ChatMessageTypeSystem, prompt)

	return ai
}

func (chat *aiChat) addMessage(role llms.ChatMessageType, text string) {
	msg := llms.MessageContent{
		Role:  role,
		Parts: make([]llms.ContentPart, 0),
	}

	part := llms.TextPart(text)
	msg.Parts = append(msg.Parts, part)
	chat.messages = append(chat.messages, msg)

	if chat.maxHst > 4 && len(chat.messages) > chat.maxHst {
		rmCount := chat.maxHst / 4
		if rmCount%2 != 0 {
			rmCount++
		}
		chat.messages = append(chat.messages[:1], chat.messages[rmCount+1:]...)
	}
}

func (chat *aiChat) addUserMessage(text string) {
	chat.addMessage(llms.ChatMessageTypeHuman, text)
}

func (chat *aiChat) addBotMessage(text string) {
	chat.addMessage(llms.ChatMessageTypeAI, text)
}

func (chat *aiChat) clear() {
	chat.messages = chat.messages[:1]
}

type AI struct {
	llm     llms.Model
	prompts map[string]string
	chats   imcache.Cache[int64, *aiChat]
	opts    []llms.CallOption
	maxHst  int
	maxInp  int
	chatExp imcache.Expiration
}

func NewAI(cfg AiConfig, exp time.Duration) (*AI, error) {
	ai := &AI{
		prompts: make(map[string]string),
		opts:    make([]llms.CallOption, 0),
		maxHst:  cfg.MaxHst,
		maxInp:  cfg.MaxInp,
		chatExp: imcache.WithSlidingExpiration(exp),
	}

	var err error
	switch cfg.Provider {
	case "openai":
		opts := make([]openai.Option, 0)
		if cfg.BaseUrl != "" {
			opts = append(opts, openai.WithBaseURL(cfg.BaseUrl))
		}
		if cfg.ApiKey != "" {
			opts = append(opts, openai.WithToken(cfg.ApiKey))
		}
		if cfg.Model != "" {
			opts = append(opts, openai.WithModel(cfg.Model))
		}
		ai.llm, err = openai.New(opts...)
	case "mistral":
		opts := make([]mistral.Option, 0)
		if cfg.BaseUrl != "" {
			opts = append(opts, mistral.WithEndpoint(cfg.BaseUrl))
		}
		if cfg.ApiKey != "" {
			opts = append(opts, mistral.WithAPIKey(cfg.ApiKey))
		}
		if cfg.Model != "" {
			opts = append(opts, mistral.WithModel(cfg.Model))
		}
		ai.llm, err = mistral.New(opts...)
	default:
		err = fmt.Errorf("unknown AI provider: %s", cfg.Provider)
	}
	if err != nil {
		return nil, err
	}

	ai.opts = append(ai.opts, llms.WithTemperature(cfg.Temp))
	ai.opts = append(ai.opts, llms.WithMaxTokens(cfg.MaxTok))
	ai.opts = append(ai.opts, llms.WithStopWords(cfg.Stop))

	return ai, nil
}

func (ai *AI) SetPrompt(langID, text string) {
	ai.prompts[langID] = text
}

func (ai *AI) StartChat(userID int64, langID string) bool {
	pmt, ok := ai.prompts[langID]
	if !ok {
		return false
	}

	chat := newAiChat(pmt, ai.maxHst)
	ai.chats.Set(userID, chat, ai.chatExp)
	return true
}

func (ai *AI) ClearChar(userID int64) error {
	chat, ok := ai.chats.Get(userID)
	if !ok {
		return fmt.Errorf("chat for user %d does not exist", userID)
	}

	chat.clear()
	return nil
}

func (ai *AI) StopChat(userID int64) {
	ai.chats.Remove(userID)
}

func (ai *AI) SendMessage(userID int64, text string) (string, error) {
	if len(text) > ai.maxInp {
		return "", fmt.Errorf("message from user %d is too long", userID)
	}

	chat, ok := ai.chats.Get(userID)
	if !ok {
		return "", fmt.Errorf("chat for user %d does not exist", userID)
	}

	chat.addUserMessage(text)
	resp, err := ai.llm.GenerateContent(context.Background(), chat.messages, ai.opts...)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no content returned from model")
	}

	if len(resp.Choices) > 1 {
		fmt.Printf("Returned choices: %d", len(resp.Choices))
	}

	reply := resp.Choices[0].Content
	if reply == "" {
		return "", fmt.Errorf("model reply content is empty")
	}

	chat.addBotMessage(reply)
	return reply, nil
}
