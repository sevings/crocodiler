package main

import (
	"crocodiler/internal/croc"
	"crocodiler/internal/helper"
	"fmt"
	"log"
	"os"
	"os/signal"
)

const botArg = "--bot"
const helpArg = "--help"
const dictArg = "--dict"

func printHelp() {
	log.Printf(
		`
Usage: %s [option]

Options are:
%s	- run Telegram bot (default).
%s	- update dictionary.
%s	- print this help message.
`, os.Args[0], botArg, dictArg, helpArg)
}

func main() {
	if len(os.Args) == 1 {
		runBot()
		return
	}

	arg := os.Args[1]
	switch arg {
	case botArg:
		runBot()
	case helpArg:
		printHelp()
	case dictArg:
		helper.UpdateDictionary()
	default:
		fmt.Printf("Unknown option: %s", arg)
	}
}

func runBot() {
	cfg, err := croc.LoadConfig()
	if err != nil {
		panic(err)
	}

	wdb := croc.NewWordDB()
	for _, lang := range cfg.Languages {
		for _, pack := range lang.WordPacks {
			err := wdb.LoadWordPack(pack.Path, lang.ID, pack.ID, pack.Part, lang.Name, pack.Name)
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

	dict, err := croc.NewDict(cfg.DictPath)
	if err != nil {
		panic(err)
	}
	defer dict.Close()

	ai, err := croc.NewAI(cfg.Ai)
	if err != nil {
		panic(err)
	}
	for _, lang := range cfg.Languages {
		if lang.Prompt != "" {
			ai.SetPrompt(lang.ID, lang.Prompt)
		}
	}

	game := croc.NewGame(db, wdb, dict, cfg.GameExp)
	bot, err := croc.NewBot(cfg, wdb, db, game, dict, ai)
	if err != nil {
		panic(err)
	}

	bot.Start()
	defer bot.Stop()

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
}
