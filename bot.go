package main

import (
	"fmt"
	tele "gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/middleware"
	"log"
	"strings"
	"time"
)

type Bot struct {
	bot  *tele.Bot
	wdb  *WordDB
	db   *DB
	game *Game

	packMenus map[string]*tele.ReplyMarkup
	langMenu  *tele.ReplyMarkup
	hostMenu  *tele.ReplyMarkup
	wordMenu  *tele.ReplyMarkup
}

func NewBot(token string, wdb *WordDB, db *DB, game *Game) (*Bot, error) {
	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 30 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		return nil, err
	}

	bot := &Bot{
		bot:  b,
		wdb:  wdb,
		db:   db,
		game: game,

		packMenus: make(map[string]*tele.ReplyMarkup),
		langMenu:  &tele.ReplyMarkup{},
		hostMenu:  &tele.ReplyMarkup{},
		wordMenu:  &tele.ReplyMarkup{},
	}

	langIDs := bot.wdb.GetLanguageIDs()
	for _, langID := range langIDs {
		packIDs, err := bot.wdb.GetWordPackIDs(langID)
		if err != nil {
			return nil, err
		}

		packRows := make([]tele.Row, 0, len(packIDs))
		packMenu := &tele.ReplyMarkup{}
		for _, packID := range packIDs {
			name, err := bot.wdb.GetWordPackName(langID, packID)
			if err != nil {
				return nil, err
			}

			btn := packMenu.Data(name, fmt.Sprintf("%s_%s", langID, packID), langID, packID)
			packRows = append(packRows, packMenu.Row(btn))
			bot.bot.Handle(&btn, bot.changeWordPack)
		}
		packMenu.Inline(packRows...)
		bot.packMenus[langID] = packMenu
	}

	langRows := make([]tele.Row, 0, len(langIDs))
	for _, langID := range langIDs {
		langName, err := wdb.GetLanguageName(langID)
		if err != nil {
			return nil, err
		}

		btn := bot.langMenu.Data(langName, fmt.Sprintf("lang_%s", langID), langID)
		langRows = append(langRows, bot.langMenu.Row(btn))
		bot.bot.Handle(&btn, bot.showWordPackMenu)
	}
	bot.langMenu.Inline(langRows...)

	hostBtn := bot.langMenu.Data("Become a host", "become_host")
	bot.hostMenu.Inline(bot.hostMenu.Row(hostBtn))

	seeBtn := bot.wordMenu.Data("See word", "see_word")
	skipBtn := bot.wordMenu.Data("Skip word", "skip_word")
	bot.wordMenu.Inline(bot.wordMenu.Row(seeBtn), bot.wordMenu.Row(skipBtn))

	bot.bot.Use(middleware.Recover())
	//bot.bot.Use(middleware.Logger())

	bot.bot.Handle("/start", bot.greet)
	bot.bot.Handle(tele.OnAddedToGroup, bot.greet)

	chatGroup := bot.bot.Group()
	chatGroup.Use(bot.checkChatGroup)

	chatGroup.Handle("/word_pack", bot.showLangMenu)
	chatGroup.Handle("/play", bot.playNewGame)
	chatGroup.Handle("/stop", bot.stopGame)
	chatGroup.Handle(&hostBtn, bot.assignGameHost)
	chatGroup.Handle(&seeBtn, bot.showWord)
	chatGroup.Handle(&skipBtn, bot.skipWord)
	chatGroup.Handle(tele.OnText, bot.checkGuess)

	return bot, nil
}

func (bot *Bot) Start() {
	go func() {
		log.Println("Starting bot...")
		bot.bot.Start()
		log.Println("Bot stopped")
	}()
}

func (bot *Bot) Stop() {
	bot.bot.Stop()
}

func (bot *Bot) checkChatGroup(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		if c.Chat().Type == tele.ChatPrivate {
			return c.Send("Add me to a group chat to play with friends.")
		}

		return next(c)
	}
}

func (bot *Bot) greet(c tele.Context) error {
	if c.Chat().Type == tele.ChatPrivate {
		return c.Send("Add me to a group.")
	}

	return c.Send("Press /play to play a game.")
}

func (bot *Bot) changeWordPack(c tele.Context) error {
	langPack := strings.Split(c.Data(), "|")
	word, err := bot.game.SetWordPack(c.Chat().ID, c.Sender().ID, langPack[0], langPack[1])
	if err != nil {
		log.Println(err)
		return c.Respond()
	}

	if word != "" {
		err = c.Respond(&tele.CallbackResponse{
			Text:      fmt.Sprintf("Your new word is \"%s\".", word),
			ShowAlert: true,
		})
		if err != nil {
			log.Println(err)
		}
	}

	bot.db.SetWordPack(c.Chat().ID, langPack[0], langPack[1])
	langName, _ := bot.wdb.GetLanguageName(langPack[0])
	packName, _ := bot.wdb.GetWordPackName(langPack[0], langPack[1])
	msg := fmt.Sprintf("Current language is <b>%s</b>.\nCurrent word pack is <b>%s</b>.",
		langName, packName)
	if word == "" {
		return c.Edit(msg, tele.ModeHTML)
	} else {
		return c.Edit(msg, bot.wordMenu, tele.ModeHTML)
	}
}

func (bot *Bot) showWordPackMenu(c tele.Context) error {
	langID := c.Data()
	langName, _ := bot.wdb.GetLanguageName(langID)
	msg := fmt.Sprintf("Current language is <b>%s</b>.", langName)
	return c.Edit(msg, bot.packMenus[langID], tele.ModeHTML)
}

func (bot *Bot) showLangMenu(c tele.Context) error {
	id := c.Chat().ID
	conf := bot.db.LoadChatConfig(id)
	var msg string
	if conf.PackID == "" {
		msg = "Please select a language and a word pack."
	} else {
		langName, _ := bot.wdb.GetLanguageName(conf.LangID)
		packName, _ := bot.wdb.GetWordPackName(conf.LangID, conf.PackID)
		msg = fmt.Sprintf("Current language is <b>%s</b>.\nCurrent word pack is <b>%s</b>.",
			langName, packName)
	}

	return c.Send(msg, bot.langMenu, tele.ModeHTML)
}

func (bot *Bot) playNewGame(c tele.Context) error {
	_, err := bot.game.Play(c.Chat().ID, c.Sender().ID)
	if err != nil {
		log.Println(err)
		msg := "Please select a language and a word pack."
		return c.Send(msg, bot.langMenu)
	}

	msg := fmt.Sprintf("%s becomes a new host.",
		printUserName(c))
	return c.Send(msg, bot.wordMenu, tele.ModeHTML)
}

func (bot *Bot) stopGame(c tele.Context) error {
	ok := bot.game.Stop(c.Chat().ID, c.Sender().ID)
	if !ok {
		return c.Send("You are not a host")
	}

	return c.Send("Game stopped.")
}

func (bot *Bot) assignGameHost(c tele.Context) error {
	word, ok := bot.game.NextWord(c.Chat().ID, c.Sender().ID)
	if !ok {
		return c.Respond(&tele.CallbackResponse{
			Text:      fmt.Sprintf("The game is active."),
			ShowAlert: true,
		})
	}

	err := c.Respond(&tele.CallbackResponse{
		Text:      fmt.Sprintf("Your word is \"%s\".", word),
		ShowAlert: true,
	})
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("%s becomes a new host.",
		printUserName(c))
	return c.Send(msg, bot.wordMenu, tele.ModeHTML)
}

func (bot *Bot) showWord(c tele.Context) error {
	var text string
	word, ok := bot.game.GetWord(c.Chat().ID, c.Sender().ID)
	if ok {
		text = fmt.Sprintf("Your word is \"%s\".", word)
	} else {
		text = "You are not a host."
	}

	return c.Respond(&tele.CallbackResponse{
		Text:      text,
		ShowAlert: true,
	})
}

func (bot *Bot) skipWord(c tele.Context) error {
	var text string
	word, ok := bot.game.SkipWord(c.Chat().ID, c.Sender().ID)
	if ok {
		text = fmt.Sprintf("Your new word is \"%s\".", word)
	} else {
		text = "You are not a host."
	}

	return c.Respond(&tele.CallbackResponse{
		Text:      text,
		ShowAlert: true,
	})
}

func (bot *Bot) checkGuess(c tele.Context) error {
	word, guessed := bot.game.CheckGuess(c.Chat().ID, c.Sender().ID, c.Text())
	if !guessed {
		return nil
	}

	msg := fmt.Sprintf("%s guessed the word <b>%s</b>.",
		printUserName(c), word)
	return c.Send(msg, bot.hostMenu, tele.ModeHTML)
}

func printUserName(c tele.Context) string {
	user := c.Sender()
	if user.LastName == "" {
		return fmt.Sprintf("<b>%s</b>",
			user.FirstName)
	}

	return fmt.Sprintf("<b>%s %s</b>",
		user.FirstName, user.LastName)
}
