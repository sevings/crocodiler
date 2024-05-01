package helper

import (
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	DictPath  string `koanf:"dict_path"`
	Languages []LanguageConfig
}

type LanguageConfig struct {
	ID        string
	Dict      DictConfig
	WordPacks []WordPackConfig `koanf:"word_packs"`
}

type DictConfig struct {
	Path  string
	Parts bool
}

type WordPackConfig struct {
	ID   string
	Path string
	Part string
}

func LoadConfig() (Config, error) {
	var kConf = koanf.New("/")

	var cfg Config

	err := kConf.Load(file.Provider("helper.toml"), toml.Parser())
	if err != nil {
		return cfg, err
	}

	err = kConf.Unmarshal("", &cfg)
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}
