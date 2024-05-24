package croc

import (
	"github.com/erni27/imcache"
	"go.uber.org/zap"
	"strings"
	"time"
)

type gameConfig struct {
	pack   *WordPack
	word   string
	def    string
	hostID int64
}

func (gc *gameConfig) isActive() bool {
	return gc.word != ""
}

func (gc *gameConfig) setNotActive() {
	gc.word = ""
	gc.def = ""
}

func (gc *gameConfig) hasDefinition() bool {
	return gc.def != ""
}

func (gc *gameConfig) checkGuess(playerID int64, guess string) bool {
	if gc.hostID == playerID {
		return false
	}

	if !gc.isActive() {
		return false
	}

	if len(guess) > len(gc.word)*2 {
		return false
	}

	guess = strings.Trim(guess, "!?,;:.^&/\\\n\t ")
	if strings.ToLower(guess) != strings.ToLower(gc.word) {
		return false
	}

	return true
}

type Game struct {
	games imcache.Cache[int64, *gameConfig]
	db    *DB
	wdb   *WordDB
	dict  *Dict
	log   *zap.SugaredLogger
	exp   imcache.Expiration
}

func NewGame(db *DB, wdb *WordDB, dict *Dict, exp time.Duration) *Game {
	if exp < time.Hour {
		exp = time.Hour
	}

	return &Game{
		db:   db,
		wdb:  wdb,
		dict: dict,
		log:  zap.L().Named("game").Sugar(),
		exp:  imcache.WithSlidingExpiration(exp),
	}
}

func (g *Game) setWord(gc *gameConfig) {
	gc.word = gc.pack.GetWord()
	def, hasDef := g.dict.FindDefinition(gc.pack.GetLangID(), gc.pack.GetPart(), gc.word)
	if hasDef {
		gc.def = def
	}

	g.log.Infow("new word",
		"word", gc.word,
		"has_def", hasDef,
		"user_id", gc.hostID)
}

func (g *Game) createConfig(chatID int64) (*gameConfig, bool) {
	gameConf, ok := g.games.Get(chatID)
	if ok {
		return gameConf, true
	}

	chatConf := g.db.LoadChatConfig(chatID)
	pack, ok := g.wdb.GetWordPack(chatConf.LangID, chatConf.PackID)
	if !ok {
		return nil, false
	}

	gameConf = &gameConfig{
		pack: pack,
	}

	g.games.Set(chatID, gameConf, g.exp)

	return gameConf, true
}

func (g *Game) Play(chatID, hostID int64) (string, bool, bool) {
	gameConf, ok := g.createConfig(chatID)
	if !ok || gameConf.isActive() {
		return "", false, false
	}

	gameConf.hostID = hostID
	g.setWord(gameConf)

	g.log.Infow("game started",
		"chat_id", chatID,
		"user_id", hostID,
		"lang_id", gameConf.pack.GetLangID(),
		"pack_id", gameConf.pack.GetPackID())

	return gameConf.word, gameConf.hasDefinition(), true
}

func (g *Game) SetWordPack(chatID, playerID int64, langID, packID string) (string, bool, bool) {
	gameConf, ok := g.games.Get(chatID)
	if !ok {
		return "", false, true
	}

	if gameConf.isActive() && gameConf.hostID != playerID {
		g.log.Warnw("player is not a host of this game",
			"chat_id", chatID,
			"user_id", playerID)
		return "", false, false
	}

	pack, ok := g.wdb.GetWordPack(langID, packID)
	if !ok {
		return "", false, false
	}

	gameConf.pack = pack

	if gameConf.isActive() {
		g.setWord(gameConf)
	}

	g.games.Set(chatID, gameConf, g.exp)

	g.log.Infow("word pack changed",
		"chat_id", chatID,
		"user_id", playerID,
		"lang_id", langID,
		"pack_id", packID)

	return gameConf.word, gameConf.hasDefinition(), true
}

func (g *Game) Stop(chatID, playerID int64) bool {
	gameConf, ok := g.games.Get(chatID)
	if !ok || !gameConf.isActive() {
		return true
	}

	if gameConf.hostID != playerID {
		return false
	}

	gameConf.setNotActive()

	g.games.Set(chatID, gameConf, g.exp)

	g.log.Infow("game stopped",
		"chat_id", chatID,
		"user_id", playerID)

	return true
}

func (g *Game) CheckGuess(chatID, playerID int64, guess string) (string, bool, bool) {
	gameConf, ok := g.games.Get(chatID)
	if !ok {
		return "", false, false
	}

	if !gameConf.checkGuess(playerID, guess) {
		return "", false, false
	}

	word := gameConf.word
	hasDef := gameConf.hasDefinition()
	gameConf.setNotActive()

	g.log.Infow("word guessed",
		"chat_id", chatID,
		"user_id", playerID)

	return word, hasDef, true
}

func (g *Game) SkipWord(chatID, playerID int64) (string, bool, bool) {
	gameConf, ok := g.games.Get(chatID)
	if !ok {
		return "", false, false
	}

	if !gameConf.isActive() {
		return "", false, false
	}

	if gameConf.hostID != playerID {
		return "", false, false
	}

	g.setWord(gameConf)

	return gameConf.word, gameConf.hasDefinition(), true
}

func (g *Game) GetWord(chatID, playerID int64) (string, bool) {
	gameConf, ok := g.games.Get(chatID)
	if !ok {
		return "", false
	}

	if !gameConf.isActive() {
		return "", false
	}

	if gameConf.hostID != playerID {
		return "", false
	}

	return gameConf.word, true
}

func (g *Game) GetDefinition(chatID, playerID int64) (string, bool) {
	gameConf, ok := g.games.Get(chatID)
	if !ok {
		return "", false
	}

	if !gameConf.isActive() {
		return "", false
	}

	if gameConf.hostID != playerID {
		return "", false
	}

	if !gameConf.hasDefinition() {
		return "", false
	}

	return gameConf.def, true
}

func (g *Game) IsActive(chatID int64) bool {
	gameConf, ok := g.games.Get(chatID)
	return ok && gameConf.isActive()
}

func (g *Game) GetActiveGames() []int64 {
	var chatIDs []int64

	games := g.games.PeekAll()
	for chatID, game := range games {
		if game.isActive() {
			chatIDs = append(chatIDs, chatID)
		}
	}

	return chatIDs
}
