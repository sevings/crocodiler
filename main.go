package main

import (
	"crocodiler/internal/croc"
	"fmt"
	"os"
	"os/signal"
)

func main() {
	cfg, err := croc.LoadConfig()
	if err != nil {
		panic(err)
	}

	wdb := croc.NewWordDB()
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

	defaultChatConfig := croc.ChatConfig{
		LangID: cfg.DefaultCfg.LangID,
		PackID: cfg.DefaultCfg.PackID,
		Locale: cfg.DefaultCfg.Locale,
	}
	db, err := croc.LoadDatabase(cfg.DBPath, defaultChatConfig)
	if err != nil {
		panic(err)
	}

	dict := croc.NewDict()
	for _, lang := range cfg.Languages {
		if lang.Dict.Path != "" {
			err = dict.LoadDict(lang.ID, lang.Dict.Path)
			if err != nil {
				panic(err)
			}
		}
	}

	game := croc.NewGame(db, wdb, dict, cfg.GameExp)
	bot, err := croc.NewBot(cfg, wdb, db, game)
	if err != nil {
		panic(err)
	}

	bot.Start()

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit

	bot.Stop()
}
