package croc

import (
	"context"
	"fmt"
	"github.com/erni27/imcache"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/mistral"
	"github.com/tmc/langchaingo/llms/openai"
	"go.uber.org/zap"
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
	log     *zap.SugaredLogger
	maxHst  int
	maxInp  int
	chatExp imcache.Expiration
}

func NewAI(cfg AiConfig, exp time.Duration) (*AI, bool) {
	ai := &AI{
		prompts: make(map[string]string),
		opts:    make([]llms.CallOption, 0),
		log:     zap.L().Named("ai").Sugar(),
		maxHst:  cfg.MaxHst,
		maxInp:  cfg.MaxInp,
		chatExp: imcache.WithSlidingExpiration(exp),
	}

	ai.log.Infow("creating AI",
		"provider", cfg.Provider,
		"base_url", cfg.BaseUrl,
		"api_key_set", cfg.ApiKey != "",
		"model", cfg.Model,
		"temperature", cfg.Temp,
		"max_tokens", cfg.MaxTok,
		"stop_words", cfg.Stop)

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
		ai.log.Error(err)
		return nil, false
	}

	ai.opts = append(ai.opts, llms.WithTemperature(cfg.Temp))
	ai.opts = append(ai.opts, llms.WithMaxTokens(cfg.MaxTok))
	ai.opts = append(ai.opts, llms.WithStopWords(cfg.Stop))

	return ai, true
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
	ai.log.Infow("chat started", "user_id", userID)
	return true
}

func (ai *AI) ClearChar(userID int64) bool {
	chat, ok := ai.chats.Get(userID)
	if !ok {
		ai.log.Warnw("chat does not exist", "user_id", userID)
		return false
	}

	chat.clear()
	ai.log.Infow("chat cleared", "user_id", userID)
	return true
}

func (ai *AI) StopChat(userID int64) {
	ai.chats.Remove(userID)
	ai.log.Infow("chat stopped", "user_id", userID)
}

func (ai *AI) SendMessage(userID int64, text string) (string, bool) {
	beginTime := time.Now().UnixNano()
	ai.log.Infow("user message",
		"user_id", userID,
		"size", len(text))
	if len(text) > ai.maxInp {
		ai.log.Warnw("message from user is too long", "user_id", userID)
		return "", false
	}

	chat, ok := ai.chats.Get(userID)
	if !ok {
		ai.log.Warnw("chat for user does not exist", "user_id", userID)
		return "", false
	}

	chat.addUserMessage(text)
	resp, err := ai.llm.GenerateContent(context.Background(), chat.messages, ai.opts...)
	if err != nil {
		ai.log.Warnw(err.Error(), "user_id", userID)
		return "", false
	}

	if len(resp.Choices) == 0 {
		ai.log.Warnw("no content returned from model", "user_id", userID)
		return "", false
	}

	if len(resp.Choices) > 1 {
		ai.log.Warnf("model returned %d choices instead of one", len(resp.Choices))
	}

	reply := resp.Choices[0].Content
	if reply == "" {
		ai.log.Warnw("model reply content is empty", "user_id", userID)
		return "", false
	}

	chat.addBotMessage(reply)

	endTime := time.Now().UnixNano()
	duration := float64(endTime-beginTime) / 1000000
	ai.log.Infow("ai message",
		"user_id", userID,
		"size", len(reply),
		"dur", fmt.Sprintf("%.2f", duration))

	return reply, true
}
