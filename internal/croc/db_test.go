package croc

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

var defaultChatCfg = ChatConfig{
	LangID: "en",
	PackID: "default",
	Locale: "en",
}

func setupTestDB(t *testing.T) *DB {
	tmpFile, err := os.CreateTemp("", "crocodiler")
	if err != nil {
		t.Fatalf("Can't create temp file for a test DB")
	}

	path := tmpFile.Name()
	_ = tmpFile.Close()
	t.Cleanup(func() { _ = os.Remove(path) })

	db, success := LoadDatabase(path, defaultChatCfg)
	require.True(t, success)
	require.NotNil(t, db)

	return db
}

func TestLoadDatabase(t *testing.T) {
	setupTestDB(t)
}

func TestLoadChatConfig(t *testing.T) {
	db := setupTestDB(t)

	tests := []struct {
		chatID int64
	}{
		{chatID: 2},
		{chatID: 1},
		{chatID: 1},
	}

	for _, tt := range tests {
		t.Run("LoadChatConfig", func(t *testing.T) {
			cfg := db.LoadChatConfig(tt.chatID)
			require.Equal(t, tt.chatID, cfg.ChatID)
			require.Equal(t, defaultChatCfg.LangID, cfg.LangID)
			require.Equal(t, defaultChatCfg.PackID, cfg.PackID)
			require.Equal(t, defaultChatCfg.Locale, cfg.Locale)
		})
	}
}

func TestSetWordPack(t *testing.T) {
	db := setupTestDB(t)

	tests := []struct {
		chatID int64
		langID string
		packID string
	}{
		{chatID: 2, langID: "fr", packID: "pack1"},
		{chatID: 1, langID: "es", packID: "pack2"},
		{chatID: 1, langID: "en", packID: "pack3"},
	}

	for _, tt := range tests {
		t.Run("SetWordPack", func(t *testing.T) {
			db.SetWordPack(tt.chatID, tt.langID, tt.packID)
			cfg := db.LoadChatConfig(tt.chatID)
			require.Equal(t, tt.langID, cfg.LangID)
			require.Equal(t, tt.packID, cfg.PackID)
		})
	}
}

func TestSetLocale(t *testing.T) {
	db := setupTestDB(t)

	tests := []struct {
		chatID int64
		locale string
	}{
		{chatID: 2, locale: "fr"},
		{chatID: 1, locale: "es"},
	}

	for _, tt := range tests {
		t.Run("SetLocale", func(t *testing.T) {
			db.SetLocale(tt.chatID, tt.locale)
			cfg := db.LoadChatConfig(tt.chatID)
			require.Equal(t, tt.locale, cfg.Locale)
		})
	}
}
