package main

import (
	"crocodiler/internal/croc"
	"crocodiler/internal/helper"
	"fmt"
	"go.uber.org/zap"
	"os"
	"os/signal"
)

const botArg = "--bot"
const helpArg = "--help"
const dictArg = "--dict"

func printHelp() {
	fmt.Printf(
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

	var zapLogger *zap.Logger
	if cfg.Release {
		zapLogger, err = zap.NewProduction(zap.WithCaller(false))
	} else {
		zapLogger, err = zap.NewDevelopment(zap.WithCaller(false))
	}
	if err != nil {
		panic(err)
	}
	defer func() { _ = zapLogger.Sync() }()

	zap.ReplaceGlobals(zapLogger)
	zap.RedirectStdLog(zapLogger)
	logger := zapLogger.Sugar()

	wdb := croc.NewWordDB()
	for _, lang := range cfg.Languages {
		for _, pack := range lang.WordPacks {
			ok := wdb.LoadWordPack(pack.Path, lang.ID, pack.ID, pack.Part, lang.Name, pack.Name)
			if !ok {
				logger.Warnw("Error loading word pack",
					"lang_id", lang.ID,
					"pack_id", pack.ID)
			}
		}
	}

	if len(wdb.GetLanguageIDs()) == 0 {
		logger.Panic("no word packs loaded")
	}

	defaultChatConfig := croc.ChatConfig{
		LangID: cfg.DefaultCfg.LangID,
		PackID: cfg.DefaultCfg.PackID,
		Locale: cfg.DefaultCfg.Locale,
	}
	db, ok := croc.LoadDatabase(cfg.DBPath, defaultChatConfig)
	if !ok {
		logger.Panic("can't load database")
	}

	dict, ok := croc.NewDict(cfg.DictPath)
	if !ok {
		logger.Panic("can't load dictionary")
	}
	defer dict.Close()

	ai, ok := croc.NewAI(cfg.Ai, cfg.GameExp)
	if !ok {
		logger.Panic("can't create AI")
	}
	for _, lang := range cfg.Languages {
		if lang.Prompt != "" {
			ai.SetPrompt(lang.ID, lang.Prompt)
		}
	}

	game := croc.NewGame(db, wdb, dict, cfg.GameExp)
	bot, ok := croc.NewBot(cfg, wdb, db, game, dict, ai)
	if !ok {
		logger.Panic("can't create bot")
	}

	bot.Start()
	defer bot.Stop()

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
}
