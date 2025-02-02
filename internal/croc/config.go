package croc

import (
	"errors"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"time"
)

type Config struct {
	TgToken      string `koanf:"tg_token"`
	DBPath       string `koanf:"db_path"`
	DictPath     string `koanf:"dict_path"`
	Release      bool
	Ai           AiConfig
	GameExp      time.Duration `koanf:"game_exp"`
	DefaultCfg   DefaultConfig `koanf:"default_cfg"`
	Translations []TranslationConfig
	Languages    []LanguageConfig
}

type AiConfig struct {
	Provider string
	BaseUrl  string `koanf:"base_url"`
	ApiKey   string `koanf:"api_key"`
	Model    string
	Temp     float64
	MaxTok   int `koanf:"max_tok"`
	MaxHst   int `koanf:"max_hst"`
	Stop     []string
	MaxInp   int `koanf:"max_inp"`
}

type DefaultConfig struct {
	Locale string
	LangID string `koanf:"lang_id"`
	PackID string `koanf:"pack_id"`
}

type TranslationConfig struct {
	Locale string
	Name   string
	Path   string
}

type LanguageConfig struct {
	ID        string
	Name      string
	Prompt    string
	WordPacks []WordPackConfig `koanf:"word_packs"`
}

type WordPackConfig struct {
	ID   string
	Name string
	Path string
	Part string
}

func LoadConfig() (Config, error) {
	var kConf = koanf.New("/")

	var cfg Config

	err := kConf.Load(file.Provider("crocodiler.toml"), toml.Parser())
	if err != nil {
		return cfg, err
	}

	err = kConf.Unmarshal("", &cfg)
	if err != nil {
		return cfg, err
	}

	if cfg.TgToken == "" {
		return cfg, errors.New("telegram token is required")
	}

	if len(cfg.Languages) == 0 {
		return cfg, errors.New("no word packs provided")
	}

	return cfg, nil
}
