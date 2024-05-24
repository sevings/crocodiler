package croc

import (
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"time"
)

type DB struct {
	db  *gorm.DB
	cfg ChatConfig
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

func LoadDatabase(path string, defaultCfg ChatConfig) (*DB, bool) {
	log := zap.L().Named("db").Sugar()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		log.Error(err)
		return nil, false
	}

	err = db.AutoMigrate(&ChatConfig{})
	if err != nil {
		log.Error(err)
		return nil, false
	}

	return &DB{
		db:  db,
		cfg: defaultCfg,
	}, true
}

func (db *DB) LoadChatConfig(chatID int64) *ChatConfig {
	var cfg ChatConfig
	db.db.Limit(1).Find(&cfg, chatID)

	if cfg.ChatID != chatID {
		cfg = db.cfg
		cfg.ChatID = chatID
		db.db.Create(&cfg)
	}

	return &cfg
}

func (db *DB) SetWordPack(chatID int64, langID, packID string) {
	tx := db.db.Model(&ChatConfig{}).Where(chatID).
		Updates(&ChatConfig{LangID: langID, PackID: packID})

	if tx.RowsAffected < 1 {
		cfg := db.cfg
		cfg.ChatID = chatID
		cfg.LangID = langID
		cfg.PackID = packID
		db.db.Create(&cfg)
	}
}

func (db *DB) SetLocale(chatID int64, locale string) {
	tx := db.db.Model(&ChatConfig{}).Where(chatID).
		Update("locale", locale)

	if tx.RowsAffected < 1 {
		cfg := db.cfg
		cfg.ChatID = chatID
		cfg.Locale = locale
		db.db.Create(&cfg)
	}
}
