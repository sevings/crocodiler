package main

import (
	"fmt"
	"os"
	"os/signal"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		panic(err)
	}

	wdb := NewWordDB()
	for _, lang := range cfg.Languages {
		for _, pack := range lang.WordPacks {
			defRe := fmt.Sprintf(lang.Dict.Pattern, pack.Part)
			err := wdb.LoadWordPack(pack.Path, lang.ID, pack.ID, defRe, lang.Name, pack.Name)
			if err != nil {
				fmt.Printf("Error loading %s/%s wordset: %s", lang.ID, pack.ID, err.Error())
			}
		}
	}

	if len(wdb.GetLanguageIDs()) == 0 {
		panic("No word packs loaded")
	}

	defaultChatConfig := ChatConfig{
		LangID: cfg.DefaultCfg.LangID,
		PackID: cfg.DefaultCfg.PackID,
		Locale: cfg.DefaultCfg.Locale,
	}
	db, err := LoadDatabase(cfg.DBPath, defaultChatConfig)
	if err != nil {
		panic(err)
	}

	dict := NewDict()
	for _, lang := range cfg.Languages {
		if lang.Dict.Path != "" {
			err = dict.LoadDict(lang.ID, lang.Dict.Path)
			if err != nil {
				panic(err)
			}
		}
	}

	game := NewGame(db, wdb, dict, cfg.GameExp)
	bot, err := NewBot(cfg, wdb, db, game)
	if err != nil {
		panic(err)
	}

	bot.Start()

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit

	bot.Stop()
}
