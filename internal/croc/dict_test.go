package croc

import (
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"os"
	"testing"
)

func setupTestDictDB(t *testing.T) *bolt.DB {
	tmpFile, err := os.CreateTemp("", "crocodiler")
	if err != nil {
		t.Fatalf("Can't create temp file for a test DB")
	}

	path := tmpFile.Name()
	_ = tmpFile.Close()
	t.Cleanup(func() { _ = os.Remove(path) })

	db, err := bolt.Open(path, 0600, nil)
	require.NoError(t, err)

	return db
}

func setupTestDict(t *testing.T, path string) *Dict {
	logger := zaptest.NewLogger(t)
	zap.ReplaceGlobals(logger)

	dict, success := NewDict(path)
	require.True(t, success)
	require.NotNil(t, dict)

	return dict
}

func TestNewDict(t *testing.T) {
	db := setupTestDictDB(t)
	path := db.Path()
	require.NoError(t, db.Close())

	setupTestDict(t, path)

	dict, ok := NewDict("/invalid/path")
	require.False(t, ok)
	require.Nil(t, dict)
}

func TestFindDefinition(t *testing.T) {
	db := setupTestDictDB(t)
	err := db.Update(func(tx *bolt.Tx) error {
		bkt, err := tx.CreateBucket([]byte("en"))
		if err != nil {
			return err
		}
		subBkt, err := bkt.CreateBucket([]byte("noun"))
		if err != nil {
			return err
		}
		return subBkt.Put([]byte("apple"), []byte("a fruit"))
	})
	require.NoError(t, err)

	path := db.Path()
	require.NoError(t, db.Close())

	dict := setupTestDict(t, path)

	def, ok := dict.FindDefinition("en", "noun", "apple")
	require.True(t, ok)
	require.Equal(t, "a fruit", def)

	def, ok = dict.FindDefinition("fr", "noun", "apple")
	require.False(t, ok)
	require.Empty(t, def)

	def, ok = dict.FindDefinition("en", "noun", "banana")
	require.False(t, ok)
	require.Empty(t, def)
}

func TestClose(t *testing.T) {
	db := setupTestDictDB(t)
	path := db.Path()
	require.NoError(t, db.Close())

	dict := setupTestDict(t, path)
	dict.Close()

	err := dict.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte("test"))
		return err
	})
	require.Error(t, err)
}
