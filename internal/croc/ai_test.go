package croc

import (
	"github.com/stretchr/testify/require"
	"github.com/tmc/langchaingo/llms"
	"go.uber.org/zap/zaptest"
	"testing"
)

func (chat *aiChat) getMessageText(i int) string {
	if i >= len(chat.messages) {
		return ""
	}

	part := chat.messages[i].Parts[0]
	text, ok := part.(llms.TextContent)
	if !ok {
		return ""
	}

	return text.Text
}

func (chat *aiChat) getMessageRole(i int) llms.ChatMessageType {
	if i >= len(chat.messages) {
		return ""
	}

	return chat.messages[i].Role
}

func (chat *aiChat) getMessageCount() int {
	return len(chat.messages)
}

func setupAiChat(t *testing.T, prompt string, maxHistory int) *aiChat {
	logger := zaptest.NewLogger(t).Sugar()
	return newAiChat(prompt, maxHistory, logger)
}

func TestNewAiChat(t *testing.T) {
	prompt := "Welcome!"
	maxHistory := 10
	chat := setupAiChat(t, prompt, maxHistory)

	require.Equal(t, 1, chat.getMessageCount())
	require.Equal(t, llms.ChatMessageTypeSystem, chat.getMessageRole(0))
	require.Equal(t, prompt, chat.getMessageText(0))
	require.Equal(t, maxHistory, chat.maxHst)
	require.NotNil(t, chat.log)
}

func TestAddMessage(t *testing.T) {
	chat := setupAiChat(t, "Welcome!", 5)

	chat.addUserMessage("Hello!")
	require.Equal(t, 2, chat.getMessageCount())
	require.Equal(t, llms.ChatMessageTypeHuman, chat.getMessageRole(1))
	require.Equal(t, "Hello!", chat.getMessageText(1))

	chat.addBotMessage("Hi there!")
	require.Equal(t, 3, chat.getMessageCount())
	require.Equal(t, llms.ChatMessageTypeAI, chat.getMessageRole(2))
	require.Equal(t, "Hi there!", chat.getMessageText(2))

	for i := 0; i < 5; i++ {
		if i%2 == 0 {
			chat.addUserMessage("User message")
		} else {
			chat.addBotMessage("Bot message")
		}
	}

	require.LessOrEqual(t, chat.getMessageCount(), chat.maxHst)
}

func TestRestart(t *testing.T) {
	chat := setupAiChat(t, "Welcome!", 5)
	chat.addUserMessage("User message")
	chat.restart("newWord")

	require.Equal(t, 1, chat.getMessageCount())
	require.Equal(t, "newWord", chat.word)
	require.NotEqual(t, uint32(0), chat.id)
}
