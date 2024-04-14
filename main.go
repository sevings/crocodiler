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
			err := wdb.LoadWordPack(pack.Path, lang.ID, pack.ID, lang.Name, pack.Name)
			if err != nil {
				fmt.Printf("Error loading %s/%s wordset: %s", lang.ID, pack.ID, err.Error())
			}
		}
	}

	if len(wdb.GetLanguageIDs()) == 0 {
		panic("No word packs loaded")
	}

	db, err := LoadDatabase(cfg.DBPath)
	if err != nil {
		panic(err)
	}

	game := NewGame(db, wdb)
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
