package main

import (
	"errors"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	TgToken   string `koanf:"tg_token"`
	DBPath    string `koanf:"db_path"`
	Languages []LanguageConfig
}

type LanguageConfig struct {
	ID        string
	Name      string
	WordPacks []WordPackConfig `koanf:"word_packs"`
}

type WordPackConfig struct {
	ID   string
	Name string
	Path string
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
