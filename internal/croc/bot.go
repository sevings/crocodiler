package croc

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"
	"golang.org/x/text/language"
	tele "gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/middleware"
	"math"
	"strings"
	"sync/atomic"
	"time"
)

var (
	btnBecomeHost  = &i18n.Message{ID: "btn_become_host", Other: "Become a host"}
	btnWhatsThat   = &i18n.Message{ID: "btn_whats_that", Other: "What is that?"}
	btnSeeWord     = &i18n.Message{ID: "btn_see_word", Other: "See word"}
	btnPeekDef     = &i18n.Message{ID: "btn_peek_definition", Other: "Peek definition"}
	btnSkipWord    = &i18n.Message{ID: "btn_skip_word", Other: "Skip word"}
	msgChangeLang  = &i18n.Message{ID: "msg_change_lang", Other: "This language is not yet supported in single player mode."}
	msgLangChanged = &i18n.Message{ID: "msg_lang_changed", Other: "Language changed."}
	msgNewWord     = &i18n.Message{ID: "msg_new_word", Other: "Your new word is \"{{.word}}\"."}
	msgCurrPack    = &i18n.Message{
		ID:    "msg_curr_pack",
		Other: "Current language is <b>{{.lang}}</b>.\nCurrent word pack is <b>{{.pack}}</b>.",
	}
	msgCurrLang   = &i18n.Message{ID: "msg_curr_lang", Other: "Current language is <b>{{.lang}}</b>."}
	msgSelectPack = &i18n.Message{ID: "msg_select_pack", Other: "Please select a language and a word pack."}
	msgAiDisclaim = &i18n.Message{
		ID: "msg_ai_disclaim",
		Other: "The text of the messages sent during the single-player game will be saved " +
			"and later used to improve performance of the bot.",
	}
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
		"When some player sends the correct guess, the game ends.\n\n" +
		"You can add me to a group and play with friends, " +
		"or you can play against the AI in single player mode. " +
		"There are several languages and difficulty levels available."}
	msgShutdown = &i18n.Message{ID: "msg_shutdown", Other: "The bot is about to update. It usually takes few minutes."}
)

type Bot struct {
	bot  *tele.Bot
	wdb  *WordDB
	db   *DB
	game *Game
	dict *Dict
	ai   *AI
	trs  map[string]*i18n.Localizer
	log  *zap.SugaredLogger

	packMenus    map[string]*tele.ReplyMarkup
	langMenu     *tele.ReplyMarkup
	wordMenus    map[string]*tele.ReplyMarkup
	wordDefMenus map[string]*tele.ReplyMarkup
	trMenu       *tele.ReplyMarkup

	startedAt      time.Time
	userGuessCount atomic.Int64
	AiGuessCount   atomic.Int64
	wordCount      atomic.Int64
}

func NewBot(cfg Config, wdb *WordDB, db *DB, game *Game, dict *Dict, ai *AI) (*Bot, bool) {
	pref := tele.Settings{
		Token:  cfg.TgToken,
		Poller: &tele.LongPoller{Timeout: 30 * time.Second},
	}

	bot := &Bot{
		wdb:  wdb,
		db:   db,
		game: game,
		dict: dict,
		ai:   ai,
		trs:  make(map[string]*i18n.Localizer),
		log:  zap.L().Named("bot").Sugar(),

		packMenus:    make(map[string]*tele.ReplyMarkup),
		langMenu:     &tele.ReplyMarkup{},
		wordMenus:    make(map[string]*tele.ReplyMarkup),
		wordDefMenus: make(map[string]*tele.ReplyMarkup),
		trMenu:       &tele.ReplyMarkup{},

		startedAt: time.Now(),
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		bot.log.Error(err)
		return nil, false
	}
	bot.bot = b

	trRows := make([]tele.Row, 0, len(cfg.Translations))
	for _, tr := range cfg.Translations {
		tag, err := language.Parse(tr.Locale)
		if err != nil {
			bot.log.Error(err)
			return nil, false
		}

		bundle := i18n.NewBundle(tag)
		bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
		_, err = bundle.LoadMessageFile(tr.Path)
		if err != nil {
			bot.log.Error(err)
			return nil, false
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
		packIDs, ok := bot.wdb.GetWordPackIDs(langID)
		if !ok {
			return nil, false
		}

		packRows := make([]tele.Row, 0, len(packIDs))
		packMenu := &tele.ReplyMarkup{}
		for _, packID := range packIDs {
			name, ok := bot.wdb.GetWordPackName(langID, packID)
			if !ok {
				return nil, false
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
		langName, ok := wdb.GetLanguageName(langID)
		if !ok {
			return nil, false
		}

		btn := bot.langMenu.Data(langName, fmt.Sprintf("lang_%s", langID), langID)
		langRows = append(langRows, bot.langMenu.Row(btn))
		bot.bot.Handle(&btn, bot.showWordPackMenu)
	}
	bot.langMenu.Inline(langRows...)

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

		bot.bot.Handle(&seeBtn, bot.showWord)
		bot.bot.Handle(&defBtn, bot.showDefinition)
		bot.bot.Handle(&skipBtn, bot.skipWord)
	}

	{
		hostMenu := &tele.ReplyMarkup{}
		hostBtn := hostMenu.Data("", "become_host")
		bot.bot.Handle(&hostBtn, bot.assignGameHost)
		whatBtn := hostMenu.Data("", "whats_that")
		bot.bot.Handle(&whatBtn, bot.showOldDefinition)
	}

	if _, ok := bot.trs[cfg.DefaultCfg.Locale]; !ok {
		bot.log.Errorw("default locale not found", "locale", cfg.DefaultCfg.Locale)
		return nil, false
	}

	if _, ok := bot.packMenus[cfg.DefaultCfg.LangID]; !ok {
		bot.log.Errorw("default language not found", "lang_id", cfg.DefaultCfg.LangID)
		return nil, false
	}

	if _, ok := bot.wdb.GetWordPack(cfg.DefaultCfg.LangID, cfg.DefaultCfg.PackID); !ok {
		bot.log.Errorw("default word pack not found", "pack_id", cfg.DefaultCfg.PackID)
		return nil, false
	}

	bot.bot.Use(middleware.Recover())
	bot.bot.Use(bot.logMessage)

	bot.bot.Handle("/start", bot.showTrMenu)
	bot.bot.Handle("/language", bot.showTrMenu)
	bot.bot.Handle(tele.OnAddedToGroup, bot.showTrMenu)
	bot.bot.Handle("/help", bot.showHelp)
	bot.bot.Handle("/play", bot.playNewGame)
	bot.bot.Handle("/stat", bot.getBotStat)

	bot.bot.Handle("/word_pack", bot.showLangMenu)
	bot.bot.Handle("/stop", bot.stopGame)
	bot.bot.Handle(tele.OnText, bot.checkGuess)

	return bot, true
}

func (bot *Bot) Start() {
	go func() {
		bot.log.Info("starting bot")
		bot.bot.Start()
		bot.log.Info("bot stopped")
	}()
}

func (bot *Bot) Stop() {
	chatIDs := bot.game.GetActiveGames()
	bot.log.Infow("stopping bot", "games", len(chatIDs))
	for _, chatID := range chatIDs {
		msg := bot.tr(msgShutdown, bot.getLocaleByChatID(chatID))
		_, err := bot.bot.Send(tele.ChatID(chatID), msg)
		if err != nil {
			bot.log.Warn(err)
		}
	}

	bot.bot.Stop()
}

func (bot *Bot) logMessage(next tele.HandlerFunc) tele.HandlerFunc {
	mention := "@" + bot.bot.Me.Username
	return func(c tele.Context) error {
		beginTime := time.Now().UnixNano()

		err := next(c)

		endTime := time.Now().UnixNano()
		duration := float64(endTime-beginTime) / 1000000

		isCmd := len(c.Text()) > 0 && c.Text()[0] == '/' && len(c.Entities()) == 1
		if c.Chat().Type == tele.ChatPrivate || strings.Contains(c.Text(), mention) || isCmd {
			var cmd string
			if isCmd {
				cmd = c.Text()
			}
			bot.log.Infow("user message",
				"chat_id", c.Chat().ID,
				"chat_type", c.Chat().Type,
				"user_id", c.Sender().ID,
				"user_name", c.Sender().Username,
				"is_cmd", isCmd,
				"cmd", cmd,
				"size", len(c.Text()),
				"dur", fmt.Sprintf("%.2f", duration),
				"err", err)
		}

		return err
	}
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
		bot.log.Warn(err)
		return lc.DefaultMessage.Other
	}

	return msg
}

func (bot *Bot) showTrMenu(c tele.Context) error {
	msg := "Select language."

	return c.Send(msg, bot.trMenu)
}

func (bot *Bot) changeTr(c tele.Context) error {
	locale := c.Data()
	bot.db.SetLocale(c.Chat().ID, locale)

	err := c.Respond(&tele.CallbackResponse{Text: bot.tr(msgLangChanged, locale)})
	if err != nil {
		return err
	}

	msg := bot.tr(msgRules, locale)
	msg += "\n\n"
	msg += bot.tr(msgHelp, locale)

	return c.Edit(msg)
}

func (bot *Bot) showHelp(c tele.Context) error {
	locale := bot.getLocale(c)

	msg := bot.tr(msgRules, locale)
	msg += "\n\n"
	msg += bot.tr(msgHelp, locale)

	return c.Send(msg)
}

func (bot *Bot) changeWordPack(c tele.Context) error {
	langPack := c.Args()
	if c.Chat().Type == tele.ChatPrivate {
		ok := bot.ai.PrepareChat(c.Chat().ID, langPack[0])
		if !ok {
			err := respondAlert(c, bot.tr(msgChangeLang, bot.getLocale(c)))
			if err != nil {
				return err
			}

			return bot.showLangMenu(c)
		}
	}

	word, hasDef, ok := bot.game.SetWordPack(c.Chat().ID, c.Sender().ID, langPack[0], langPack[1])
	if !ok {
		return c.Respond()
	}

	if word != "" && c.Chat().Type == tele.ChatPrivate {
		bot.ai.RestartChat(c.Chat().ID, word)
	}

	locale := bot.getLocale(c)
	if word != "" {
		lc := &i18n.LocalizeConfig{
			DefaultMessage: msgNewWord,
			TemplateData: map[string]string{
				"word": word,
			},
		}
		err := respondAlert(c, bot.trCfg(lc, locale))
		if err != nil {
			bot.log.Warn(err)
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

func (bot *Bot) getLangMessage(c tele.Context) string {
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

	return msg
}

func (bot *Bot) showLangMenu(c tele.Context) error {
	msg := bot.getLangMessage(c)

	return c.Send(msg, bot.langMenu, tele.ModeHTML)
}

func (bot *Bot) playNewGame(c tele.Context) error {
	langMsg := bot.getLangMessage(c)
	err := c.Send(langMsg, tele.ModeHTML)
	if err != nil {
		return err
	}

	if c.Chat().Type == tele.ChatPrivate {
		cfg := bot.db.LoadChatConfig(c.Chat().ID)
		started := bot.ai.PrepareChat(c.Chat().ID, cfg.LangID)
		if !started {
			return c.Send(bot.tr(msgChangeLang, bot.getLocale(c)))
		}
	}

	locale := bot.getLocale(c)
	word, hasDef, ok := bot.game.Play(c.Chat().ID, c.Sender().ID)
	if !ok {
		msg := bot.tr(msgGameActive, locale)
		return c.Send(msg)
	}

	var msg string
	if c.Chat().Type == tele.ChatPrivate {
		if word != "" {
			bot.ai.RestartChat(c.Chat().ID, word)
		}

		msg = bot.tr(msgAiDisclaim, locale)
	} else {
		lc := &i18n.LocalizeConfig{
			DefaultMessage: msgNewHost,
			TemplateData: map[string]string{
				"name": printUserName(c.Sender()),
			},
		}
		msg = bot.trCfg(lc, locale)
	}

	if hasDef {
		return c.Send(msg, bot.wordDefMenus[locale], tele.ModeHTML)
	}
	return c.Send(msg, bot.wordMenus[locale], tele.ModeHTML)
}

func (bot *Bot) stopGame(c tele.Context) error {
	if c.Chat().Type == tele.ChatPrivate {
		bot.ai.StopChat(c.Chat().ID)
	}

	ok := bot.game.Stop(c.Chat().ID, c.Sender().ID)
	if !ok {
		return c.Send(bot.tr(msgNotHost, bot.getLocale(c)))
	}

	return c.Send(bot.tr(msgGameStopped, bot.getLocale(c)))
}

func (bot *Bot) assignGameHost(c tele.Context) error {
	cfg := bot.db.LoadChatConfig(c.Chat().ID)
	word, hasDef, ok := bot.game.Play(c.Chat().ID, c.Sender().ID)
	if !ok {
		return respondAlert(c, bot.tr(msgGameActive, cfg.Locale))
	}

	if c.Chat().Type == tele.ChatPrivate {
		bot.ai.PrepareChat(c.Chat().ID, cfg.LangID)
		bot.ai.RestartChat(c.Chat().ID, word)
	}

	lc := &i18n.LocalizeConfig{
		DefaultMessage: msgYourWord,
		TemplateData: map[string]string{
			"word": word,
		},
	}
	err := respondAlert(c, bot.trCfg(lc, cfg.Locale))
	if err != nil {
		return err
	}

	menu := c.Message().ReplyMarkup
	rowCount := len(menu.InlineKeyboard)
	if rowCount > 0 {
		menu.InlineKeyboard = menu.InlineKeyboard[1:]
		err = c.Edit(menu, tele.ModeHTML)
		if err != nil {
			return err
		}
	}

	lc = &i18n.LocalizeConfig{
		DefaultMessage: msgNewHost,
		TemplateData: map[string]string{
			"name": printUserName(c.Sender()),
		},
	}
	msg := bot.trCfg(lc, cfg.Locale)
	if hasDef {
		return c.Send(msg, bot.wordDefMenus[cfg.Locale], tele.ModeHTML)
	}
	return c.Send(msg, bot.wordMenus[cfg.Locale], tele.ModeHTML)
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

	return respondAlert(c, text)
}

func truncateDefinition(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}

	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}

	splitters := []rune{'\n', '.', ' '}
	for _, s := range splitters {
		lastIdx := -1
		for i := maxLen - 1; i >= 0; i-- {
			if runes[i] == s {
				lastIdx = i
				break
			}
		}

		if lastIdx != -1 && (lastIdx >= maxLen/2 || s == splitters[len(splitters)-1]) {
			runes = runes[:lastIdx]
			break
		}
	}

	if len(runes) >= maxLen {
		runes = runes[:maxLen-1]
	}

	return string(runes) + "…"
}

func (bot *Bot) showDefinition(c tele.Context) error {
	text, hasDef := bot.game.GetDefinition(c.Chat().ID, c.Sender().ID)
	if hasDef {
		text = truncateDefinition(text, 200)
	} else {
		text = bot.tr(msgNotHost, bot.getLocale(c))
	}

	return respondAlert(c, text)
}

func (bot *Bot) showOldDefinition(c tele.Context) error {
	langPartWord := c.Args()
	if len(langPartWord) < 3 {
		return c.Respond()
	}

	def, ok := bot.dict.FindDefinition(langPartWord[0], langPartWord[1], langPartWord[2])
	if !ok {
		return c.Respond()
	}

	def = truncateDefinition(def, 1000)
	def = langPartWord[2] + "\n\n" + def
	err := c.Send(def)
	if err != nil {
		return err
	}

	menu := c.Message().ReplyMarkup
	rowCount := len(menu.InlineKeyboard)
	if rowCount > 0 {
		menu.InlineKeyboard = menu.InlineKeyboard[:rowCount-1]
		err = c.Edit(menu, tele.ModeHTML)
		if err != nil {
			return err
		}
	}

	return c.Respond()
}

func (bot *Bot) skipWord(c tele.Context) error {
	locale := bot.getLocale(c)
	word, hasDef, ok := bot.game.SkipWord(c.Chat().ID, c.Sender().ID)
	if !ok {
		text := bot.tr(msgNotHost, locale)
		return respondAlert(c, text)
	}

	lc := &i18n.LocalizeConfig{
		DefaultMessage: msgNewWord,
		TemplateData: map[string]string{
			"word": word,
		},
	}
	text := bot.trCfg(lc, locale)

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

	if c.Chat().Type == tele.ChatPrivate {
		bot.ai.RestartChat(c.Chat().ID, word)
	}

	return respondAlert(c, text)
}

func (bot *Bot) checkGuess(c tele.Context) error {
	if !bot.game.IsActive(c.Chat().ID) {
		return nil
	}

	guess := c.Text()
	guesser := c.Sender()

	if c.Chat().Type == tele.ChatPrivate {
		bot.AiGuessCount.Add(1)

		text := c.Message().Text
		if c.Message().ReplyTo != nil {
			replied := c.Message().ReplyTo.Text
			replied = "> " + strings.ReplaceAll(replied, "\n", "\n> ")
			text = replied + "\n\n" + text
		}
		reply, ok := bot.ai.SendMessage(c.Chat().ID, text)
		if !ok {
			return nil
		}

		err := c.Send(reply)
		if err != nil {
			return err
		}

		guess = reply
		guesser = bot.bot.Me
	} else {
		bot.userGuessCount.Add(1)
	}

	word, hasDef, guessed := bot.game.CheckGuess(c.Chat().ID, guesser.ID, guess)
	if !guessed {
		return nil
	}

	bot.wordCount.Add(1)

	locale := bot.getLocale(c)
	lc := &i18n.LocalizeConfig{
		DefaultMessage: msgGuessedWord,
		TemplateData: map[string]string{
			"name": printUserName(guesser),
			"word": word,
		},
	}
	msg := bot.trCfg(lc, locale)

	hostMenu := &tele.ReplyMarkup{}
	hostBtn := hostMenu.Data(bot.tr(btnBecomeHost, locale), "become_host")
	if hasDef {
		cfg := bot.db.LoadChatConfig(c.Chat().ID)
		pack, ok := bot.wdb.GetWordPack(cfg.LangID, cfg.PackID)
		if ok {
			whatBtn := hostMenu.Data(bot.tr(btnWhatsThat, locale), "whats_that",
				pack.langID, pack.part, word)
			hostMenu.Inline(hostMenu.Row(hostBtn), hostMenu.Row(whatBtn))
		} else {
			hostMenu.Inline(hostMenu.Row(hostBtn))
		}
	} else {
		hostMenu.Inline(hostMenu.Row(hostBtn))
	}

	return c.Send(msg, hostMenu, tele.ModeHTML)
}

func (bot *Bot) getBotStat(c tele.Context) error {
	var msg strings.Builder

	addF64 := func(title string, value float64) {
		if math.IsInf(value, 0) || math.IsNaN(value) {
			value = 0
		}

		msg.WriteString(fmt.Sprintf("%s: %.2f\n", title, value))
	}

	addI64 := func(title string, value int64) {
		msg.WriteString(fmt.Sprintf("%s: %d\n", title, value))
	}

	uptimeDays := time.Now().Sub(bot.startedAt).Hours() / 24
	addF64("Uptime (days)", uptimeDays)

	aiGuessCnt := bot.AiGuessCount.Load()
	addI64("Total AI guesses", aiGuessCnt)

	userGuessCnt := bot.userGuessCount.Load()
	addI64("Total user guesses", userGuessCnt)

	wordCnt := bot.wordCount.Load()
	addI64("Total words guessed", wordCnt)

	if uptimeDays > 0.1 {
		avgAiGuessCnt := float64(aiGuessCnt) / uptimeDays
		addF64("AI guesses per day", avgAiGuessCnt)

		avgUserGuessCnt := float64(userGuessCnt) / uptimeDays
		addF64("User guesses per day", avgUserGuessCnt)

		avgWordCnt := float64(wordCnt) / uptimeDays
		addF64("Words guessed per day", avgWordCnt)
	}

	groupChatCnt := bot.db.GetChatCount()
	addI64("Total chats", groupChatCnt)

	return c.Reply(msg.String(), tele.ModeHTML)
}

func printUserName(user *tele.User) string {
	if user.LastName == "" {
		return fmt.Sprintf("<b>%s</b>",
			user.FirstName)
	}

	return fmt.Sprintf("<b>%s %s</b>",
		user.FirstName, user.LastName)
}

func respondAlert(c tele.Context, text string) error {
	return c.Respond(&tele.CallbackResponse{
		Text:      text,
		ShowAlert: true,
	})
}
