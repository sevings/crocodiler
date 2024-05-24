package croc

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type wordPackCfg struct {
	langID   string
	packID   string
	part     string
	langName string
	packName string
}

var defaultWordPackCfg = wordPackCfg{
	langID:   "en",
	packID:   "pack1",
	part:     "noun",
	langName: "en",
	packName: "pack1",
}

func setupTestWordDB(t *testing.T) *WordDB {
	tmpFile, err := os.CreateTemp("", "crocodiler")
	if err != nil {
		t.Fatalf("Can't create temp file for a word pack")
	}

	path := tmpFile.Name()
	_ = tmpFile.Close()
	t.Cleanup(func() { _ = os.Remove(path) })

	err = os.WriteFile(path, []byte("word1\nword2\n"), 0644)
	require.NoError(t, err)

	db := NewWordDB()
	cfg := defaultWordPackCfg
	ok := db.LoadWordPack(path, cfg.langID, cfg.packID, cfg.part, cfg.langName, cfg.packName)
	require.True(t, ok)

	return db
}

func TestWordDB_LoadWordPack(t *testing.T) {
	setupTestWordDB(t)
}

func TestWordDB_GetLanguageIDs(t *testing.T) {
	db := setupTestWordDB(t)
	require.Equal(t, []string{defaultWordPackCfg.langID}, db.GetLanguageIDs())
}

func TestWordDB_GetWordPackIDs(t *testing.T) {
	db := setupTestWordDB(t)
	packIDs, ok := db.GetWordPackIDs(defaultWordPackCfg.langID)
	require.True(t, ok)
	require.Equal(t, []string{defaultWordPackCfg.packID}, packIDs)
}

func TestWordDB_GetLanguageName(t *testing.T) {
	db := setupTestWordDB(t)
	name, ok := db.GetLanguageName(defaultWordPackCfg.langID)
	require.True(t, ok)
	require.Equal(t, defaultWordPackCfg.langName, name)
}

func TestWordDB_GetWordPackName(t *testing.T) {
	db := setupTestWordDB(t)
	name, ok := db.GetWordPackName(defaultWordPackCfg.langID, defaultWordPackCfg.packID)
	require.True(t, ok)
	require.Equal(t, defaultWordPackCfg.packName, name)
}

func TestWordDB_GetWordPack(t *testing.T) {
	db := setupTestWordDB(t)
	pack, ok := db.GetWordPack(defaultWordPackCfg.langID, defaultWordPackCfg.packID)
	require.True(t, ok)
	require.Equal(t, defaultWordPackCfg.langID, pack.GetLangID())
	require.Equal(t, defaultWordPackCfg.packID, pack.GetPackID())
	require.Equal(t, defaultWordPackCfg.part, pack.GetPart())
	require.NotEmpty(t, pack.GetWord())
}
