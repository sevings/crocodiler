package croc

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	tele "gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/middleware"
	"log"
	"strings"
	"time"
)

var (
	btnBecomeHost  = &i18n.Message{ID: "btn_become_host", Other: "Become a host"}
	btnSeeWord     = &i18n.Message{ID: "btn_see_word", Other: "See word"}
	btnPeekDef     = &i18n.Message{ID: "btn_peek_definition", Other: "Peek definition"}
	btnSkipWord    = &i18n.Message{ID: "btn_skip_word", Other: "Skip word"}
	msgAddToGroup  = &i18n.Message{ID: "msg_add_to_group", Other: "Add me to a group chat to play with friends."}
	msgLangChanged = &i18n.Message{ID: "msg_lang_changed", Other: "Language changed."}
	msgNewWord     = &i18n.Message{ID: "msg_new_word", Other: "Your new word is \"{{.word}}\"."}
	msgCurrPack    = &i18n.Message{
		ID:    "msg_curr_pack",
		Other: "Current language is <b>{{.lang}}</b>.\nCurrent word pack is <b>{{.pack}}</b>.",
	}
	msgCurrLang    = &i18n.Message{ID: "msg_curr_lang", Other: "Current language is <b>{{.lang}}</b>."}
	msgSelectPack  = &i18n.Message{ID: "msg_select_pack", Other: "Please select a language and a word pack."}
	msgNewHost     = &i18n.Message{ID: "msg_new_host", Other: "{{.name}} becomes a new host."}
	msgNotHost     = &i18n.Message{ID: "msg_not_host", Other: "You are not the current host."}
	msgGameStopped = &i18n.Message{ID: "msg_game_stopped", Other: "Game stopped."}
	msgGameActive  = &i18n.Message{ID: "msg_game_active", Other: "Game is active."}
	msgYourWord    = &i18n.Message{ID: "msg_your_word", Other: "Your word is \"{{.word}}\"."}
	msgGuessedWord = &i18n.Message{ID: "msg_guessed_word", Other: "{{.name}} guessed the word <b>{{.word}}</b>"}
	msgHelp        = &i18n.Message{ID: "msg_help", Other: "" +
		"Send /play to start a new game.\n" +
		"Send /word_pack to select a word pack.\n" +
		"Send /language to change interface language.\n" +
		"Send /stop to stop the current game.\n"}
	msgRules = &i18n.Message{ID: "msg_rules", Other: "Hello! " +
		"I am a bot created to play a word guessing game.\n\n" +
		"The rules are simple. There is a game host and multiple players. " +
		"The game host receives a random word and explains it to the other participants " +
		"without using the same root words. " +
		"Then, everyone tries to guess the word. " +
		"When some player sends the correct guess, the game ends. " +
		"Anyone can take on the role of the new game host, and we start from the beginning."}
	msgShutdown = &i18n.Message{ID: "msg_shutdown", Other: "The bot is about to update. It usually takes few minutes."}
)

type Bot struct {
	bot  *tele.Bot
	wdb  *WordDB
	db   *DB
	game *Game
	trs  map[string]*i18n.Localizer

	packMenus    map[string]*tele.ReplyMarkup
	langMenu     *tele.ReplyMarkup
	hostMenus    map[string]*tele.ReplyMarkup
	wordMenus    map[string]*tele.ReplyMarkup
	wordDefMenus map[string]*tele.ReplyMarkup
	trMenu       *tele.ReplyMarkup
}

func NewBot(cfg Config, wdb *WordDB, db *DB, game *Game) (*Bot, error) {
	pref := tele.Settings{
		Token:  cfg.TgToken,
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
		trs:  make(map[string]*i18n.Localizer),

		packMenus:    make(map[string]*tele.ReplyMarkup),
		langMenu:     &tele.ReplyMarkup{},
		hostMenus:    make(map[string]*tele.ReplyMarkup),
		wordMenus:    make(map[string]*tele.ReplyMarkup),
		wordDefMenus: make(map[string]*tele.ReplyMarkup),
		trMenu:       &tele.ReplyMarkup{},
	}

	chatGroup := bot.bot.Group()
	chatGroup.Use(bot.checkChatGroup)

	trRows := make([]tele.Row, 0, len(cfg.Translations))
	for _, tr := range cfg.Translations {
		tag, err := language.Parse(tr.Locale)
		if err != nil {
			return nil, err
		}

		bundle := i18n.NewBundle(tag)
		bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
		_, err = bundle.LoadMessageFile(tr.Path)
		if err != nil {
			return nil, err
		}

		locale := i18n.NewLocalizer(bundle)
		bot.trs[tr.Locale] = locale

		btn := bot.trMenu.Data(tr.Name, fmt.Sprintf("tr_%s", tr.Locale), tr.Locale)
		trRows = append(trRows, bot.trMenu.Row(btn))
		bot.bot.Handle(&btn, bot.changeTr)
	}
	bot.trMenu.Inline(trRows...)

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

	for _, tr := range cfg.Translations {
		hostMenu := &tele.ReplyMarkup{}
		hostBtn := hostMenu.Data(bot.tr(btnBecomeHost, tr.Locale), "become_host")
		hostMenu.Inline(hostMenu.Row(hostBtn))
		bot.hostMenus[tr.Locale] = hostMenu

		chatGroup.Handle(&hostBtn, bot.assignGameHost)
	}

	for _, tr := range cfg.Translations {
		wordMenu := &tele.ReplyMarkup{}
		seeBtn := wordMenu.Data(bot.tr(btnSeeWord, tr.Locale), "see_word")
		defBtn := wordMenu.Data(bot.tr(btnPeekDef, tr.Locale), "see_def")
		skipBtn := wordMenu.Data(bot.tr(btnSkipWord, tr.Locale), "skip_word")
		wordMenu.Inline(wordMenu.Row(seeBtn), wordMenu.Row(skipBtn))
		bot.wordMenus[tr.Locale] = wordMenu

		wordDefMenu := &tele.ReplyMarkup{}
		wordDefMenu.Inline(wordDefMenu.Row(seeBtn), wordDefMenu.Row(defBtn), wordDefMenu.Row(skipBtn))
		bot.wordDefMenus[tr.Locale] = wordDefMenu

		chatGroup.Handle(&seeBtn, bot.showWord)
		chatGroup.Handle(&defBtn, bot.showDefinition)
		chatGroup.Handle(&skipBtn, bot.skipWord)
	}

	if _, ok := bot.trs[cfg.DefaultCfg.Locale]; !ok {
		return nil, fmt.Errorf("default locale '%s' not found", cfg.DefaultCfg.Locale)
	}

	if _, ok := bot.packMenus[cfg.DefaultCfg.LangID]; !ok {
		return nil, fmt.Errorf("default word pack language '%s' not found", cfg.DefaultCfg.LangID)
	}

	if _, err := bot.wdb.GetWordPack(cfg.DefaultCfg.LangID, cfg.DefaultCfg.PackID); err != nil {
		return nil, fmt.Errorf("default word pack '%s' not found", cfg.DefaultCfg.PackID)
	}

	bot.bot.Use(middleware.Recover())
	//bot.bot.Use(middleware.Logger())

	bot.bot.Handle("/start", bot.showTrMenu)
	bot.bot.Handle("/language", bot.showTrMenu)
	bot.bot.Handle(tele.OnAddedToGroup, bot.showTrMenu)
	bot.bot.Handle("/help", bot.showHelp)

	chatGroup.Handle("/word_pack", bot.showLangMenu)
	chatGroup.Handle("/play", bot.playNewGame)
	chatGroup.Handle("/stop", bot.stopGame)
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
	chatIDs := bot.game.GetActiveGames()
	for _, chatID := range chatIDs {
		msg := bot.tr(msgShutdown, bot.getLocaleByChatID(chatID))
		_, err := bot.bot.Send(tele.ChatID(chatID), msg)
		if err != nil {
			log.Println(err)
		}
	}

	bot.bot.Stop()
}

func (bot *Bot) getLocale(c tele.Context) string {
	return bot.getLocaleByChatID(c.Chat().ID)
}

func (bot *Bot) getLocaleByChatID(chatID int64) string {
	return bot.db.LoadChatConfig(chatID).Locale
}

func (bot *Bot) tr(msg *i18n.Message, locale string) string {
	return bot.trCfg(&i18n.LocalizeConfig{DefaultMessage: msg}, locale)
}

func (bot *Bot) trCfg(lc *i18n.LocalizeConfig, locale string) string {
	msg, err := bot.trs[locale].Localize(lc)
	if err != nil {
		log.Println(err)
		return lc.DefaultMessage.Other
	}

	return msg
}

func (bot *Bot) checkChatGroup(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		if c.Chat().Type == tele.ChatPrivate {
			return c.Send(bot.tr(msgAddToGroup, bot.getLocale(c)))
		}

		return next(c)
	}
}

func (bot *Bot) showTrMenu(c tele.Context) error {
	msg := "Select language."

	return c.Send(msg, bot.trMenu)
}

func (bot *Bot) changeTr(c tele.Context) error {
	locale := c.Data()
	bot.db.setLocale(c.Chat().ID, locale)

	err := c.Respond(&tele.CallbackResponse{Text: bot.tr(msgLangChanged, locale)})
	if err != nil {
		return err
	}

	msg := bot.tr(msgRules, locale)
	msg += "\n\n"

	if c.Chat().Type == tele.ChatPrivate {
		msg += bot.tr(msgAddToGroup, locale)
	} else {
		msg += bot.tr(msgHelp, locale)
	}

	return c.Edit(msg)
}

func (bot *Bot) showHelp(c tele.Context) error {
	locale := bot.getLocale(c)

	msg := bot.tr(msgRules, locale)
	msg += "\n\n"

	if c.Chat().Type == tele.ChatPrivate {
		msg += bot.tr(msgAddToGroup, locale)
	} else {
		msg += bot.tr(msgHelp, locale)
	}

	return c.Send(msg)
}

func (bot *Bot) changeWordPack(c tele.Context) error {
	langPack := strings.Split(c.Data(), "|")
	word, hasDef, err := bot.game.SetWordPack(c.Chat().ID, c.Sender().ID, langPack[0], langPack[1])
	if err != nil {
		log.Println(err)
		return c.Respond()
	}

	locale := bot.getLocale(c)
	if word != "" {
		lc := &i18n.LocalizeConfig{
			DefaultMessage: msgNewWord,
			TemplateData: map[string]string{
				"word": word,
			},
		}
		err = c.Respond(&tele.CallbackResponse{
			Text:      bot.trCfg(lc, locale),
			ShowAlert: true,
		})
		if err != nil {
			log.Println(err)
		}
	}

	bot.db.SetWordPack(c.Chat().ID, langPack[0], langPack[1])
	langName, _ := bot.wdb.GetLanguageName(langPack[0])
	packName, _ := bot.wdb.GetWordPackName(langPack[0], langPack[1])
	lc := &i18n.LocalizeConfig{
		DefaultMessage: msgCurrPack,
		TemplateData: map[string]string{
			"lang": langName,
			"pack": packName,
		},
	}
	if word == "" {
		return c.Edit(bot.trCfg(lc, locale), tele.ModeHTML)
	} else if hasDef {
		return c.Edit(bot.trCfg(lc, locale), bot.wordDefMenus[locale], tele.ModeHTML)
	} else {
		return c.Edit(bot.trCfg(lc, locale), bot.wordMenus[locale], tele.ModeHTML)
	}
}

func (bot *Bot) showWordPackMenu(c tele.Context) error {
	langID := c.Data()
	langName, _ := bot.wdb.GetLanguageName(langID)
	lc := &i18n.LocalizeConfig{
		DefaultMessage: msgCurrLang,
		TemplateData: map[string]string{
			"lang": langName,
		},
	}
	return c.Edit(bot.trCfg(lc, bot.getLocale(c)), bot.packMenus[langID], tele.ModeHTML)
}

func (bot *Bot) showLangMenu(c tele.Context) error {
	id := c.Chat().ID
	conf := bot.db.LoadChatConfig(id)
	var msg string
	if conf.PackID == "" {
		msg = bot.tr(msgSelectPack, bot.getLocale(c))
	} else {
		langName, _ := bot.wdb.GetLanguageName(conf.LangID)
		packName, _ := bot.wdb.GetWordPackName(conf.LangID, conf.PackID)
		lc := &i18n.LocalizeConfig{
			DefaultMessage: msgCurrPack,
			TemplateData: map[string]string{
				"lang": langName,
				"pack": packName,
			},
		}
		msg = bot.trCfg(lc, bot.getLocale(c))
	}

	return c.Send(msg, bot.langMenu, tele.ModeHTML)
}

func (bot *Bot) playNewGame(c tele.Context) error {
	locale := bot.getLocale(c)
	hasDef, err := bot.game.Play(c.Chat().ID, c.Sender().ID)
	if err != nil {
		log.Println(err)
		msg := bot.tr(msgSelectPack, locale)
		return c.Send(msg, bot.langMenu)
	}

	lc := &i18n.LocalizeConfig{
		DefaultMessage: msgNewHost,
		TemplateData: map[string]string{
			"name": printUserName(c),
		},
	}
	msg := bot.trCfg(lc, locale)
	if hasDef {
		return c.Send(msg, bot.wordDefMenus[locale], tele.ModeHTML)
	}
	return c.Send(msg, bot.wordMenus[locale], tele.ModeHTML)
}

func (bot *Bot) stopGame(c tele.Context) error {
	ok := bot.game.Stop(c.Chat().ID, c.Sender().ID)
	if !ok {
		return c.Send(bot.tr(msgNotHost, bot.getLocale(c)))
	}

	return c.Send(bot.tr(msgGameStopped, bot.getLocale(c)))
}

func (bot *Bot) assignGameHost(c tele.Context) error {
	locale := bot.getLocale(c)
	word, hasDef, ok := bot.game.NextWord(c.Chat().ID, c.Sender().ID)
	if !ok {
		return c.Respond(&tele.CallbackResponse{
			Text:      bot.tr(msgGameActive, locale),
			ShowAlert: true,
		})
	}

	lc := &i18n.LocalizeConfig{
		DefaultMessage: msgYourWord,
		TemplateData: map[string]string{
			"word": word,
		},
	}
	err := c.Respond(&tele.CallbackResponse{
		Text:      bot.trCfg(lc, locale),
		ShowAlert: true,
	})
	if err != nil {
		return err
	}

	lc = &i18n.LocalizeConfig{
		DefaultMessage: msgNewHost,
		TemplateData: map[string]string{
			"name": printUserName(c),
		},
	}
	msg := bot.trCfg(lc, locale)
	if hasDef {
		return c.Send(msg, bot.wordDefMenus[locale], tele.ModeHTML)
	}
	return c.Send(msg, bot.wordMenus[locale], tele.ModeHTML)
}

func (bot *Bot) showWord(c tele.Context) error {
	var text string
	word, ok := bot.game.GetWord(c.Chat().ID, c.Sender().ID)
	if ok {
		lc := &i18n.LocalizeConfig{
			DefaultMessage: msgYourWord,
			TemplateData: map[string]string{
				"word": word,
			},
		}
		text = bot.trCfg(lc, bot.getLocale(c))
	} else {
		text = bot.tr(msgNotHost, bot.getLocale(c))
	}

	return c.Respond(&tele.CallbackResponse{
		Text:      text,
		ShowAlert: true,
	})
}

func truncateDefinition(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}

	text = text[:maxLen-1]
	lastIdx := strings.LastIndexByte(text, '\n')
	if lastIdx < maxLen/2 {
		lastIdx = strings.LastIndexByte(text, '.')
		if lastIdx < maxLen/2 {
			lastIdx = strings.LastIndexByte(text, ' ')
			if lastIdx < 0 {
				lastIdx = maxLen - 1
			}
		}
	}

	return text[:lastIdx] + "…"
}

func (bot *Bot) showDefinition(c tele.Context) error {
	text, hasDef := bot.game.GetDefinition(c.Chat().ID, c.Sender().ID)
	if hasDef {
		text = truncateDefinition(text, 200)
	} else {
		text = bot.tr(msgNotHost, bot.getLocale(c))
	}

	return c.Respond(&tele.CallbackResponse{
		Text:      text,
		ShowAlert: true,
	})
}

func (bot *Bot) skipWord(c tele.Context) error {
	var text string
	locale := bot.getLocale(c)
	word, hasDef, ok := bot.game.SkipWord(c.Chat().ID, c.Sender().ID)
	if ok {
		lc := &i18n.LocalizeConfig{
			DefaultMessage: msgNewWord,
			TemplateData: map[string]string{
				"word": word,
			},
		}
		text = bot.trCfg(lc, locale)
	} else {
		text = bot.tr(msgNotHost, locale)
	}

	oldHasDef := len(c.Message().ReplyMarkup.InlineKeyboard) > 2
	if oldHasDef != hasDef {
		var err error
		if hasDef {
			err = c.Edit(bot.wordDefMenus[locale], tele.ModeHTML)
		} else {
			err = c.Edit(bot.wordMenus[locale], tele.ModeHTML)
		}
		if err != nil {
			return err
		}
	}

	return c.Respond(&tele.CallbackResponse{
		Text:      text,
		ShowAlert: true,
	})
}

func (bot *Bot) checkGuess(c tele.Context) error {
	word, def, guessed := bot.game.CheckGuess(c.Chat().ID, c.Sender().ID, c.Text())
	if !guessed {
		return nil
	}

	if def != "" {
		def = truncateDefinition(def, 1000)
		def = word + "\n\n" + def
		err := c.Send(def)
		if err != nil {
			return err
		}
	}

	locale := bot.getLocale(c)
	lc := &i18n.LocalizeConfig{
		DefaultMessage: msgGuessedWord,
		TemplateData: map[string]string{
			"name": printUserName(c),
			"word": word,
		},
	}
	msg := bot.trCfg(lc, locale)
	return c.Send(msg, bot.hostMenus[locale], tele.ModeHTML)
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
