package main

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"time"
)

type DB struct {
	db *gorm.DB
}

type ChatConfig struct {
	ChatID    int64 `gorm:"primaryKey;autoIncrement:false"`
	LangID    string
	PackID    string
	Locale    string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt
}

func LoadDatabase(path string) (*DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&ChatConfig{})
	if err != nil {
		return nil, err
	}

	return &DB{
		db: db,
	}, nil
}

func (db *DB) LoadChatConfig(chatID int64) *ChatConfig {
	var cfg ChatConfig
	db.db.Limit(1).Find(&cfg, chatID)

	if cfg.ChatID != chatID {
		cfg.ChatID = chatID
		db.db.Create(&cfg)
	}

	return &cfg
}

func (db *DB) SetWordPack(chatID int64, langID, packID string) {
	db.db.Model(&ChatConfig{}).Where(chatID).
		Updates(&ChatConfig{LangID: langID, PackID: packID})
}

func (db *DB) setLocale(chatID int64, locale string) {
	db.db.Model(&ChatConfig{}).Where(chatID).
		Update("locale", locale)
}
