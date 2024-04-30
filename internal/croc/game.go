package croc

import (
	"errors"
	"github.com/erni27/imcache"
	"log"
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
		exp:  imcache.WithSlidingExpiration(exp),
	}
}

func (g *Game) setWord(gc *gameConfig) {
	gc.word = gc.pack.GetWord()
	def, err := g.dict.FindDefinition(gc.pack.GetLangID(), gc.pack.GetPart(), gc.word)
	if err != nil {
		log.Println(err)
	}
	gc.def = def
}

func (g *Game) Play(chatID, hostID int64) (bool, error) {
	chatConf := g.db.LoadChatConfig(chatID)
	if chatConf.PackID == "" {
		return false, errors.New("word pack is not chosen")
	}

	pack, err := g.wdb.GetWordPack(chatConf.LangID, chatConf.PackID)
	if err != nil {
		return false, err
	}

	gameConf := &gameConfig{
		pack:   pack,
		hostID: hostID,
	}

	g.setWord(gameConf)

	g.games.Set(chatID, gameConf, g.exp)

	return gameConf.hasDefinition(), nil
}

func (g *Game) SetWordPack(chatID, playerID int64, langID, packID string) (string, bool, error) {
	gameConf, ok := g.games.Get(chatID)
	if !ok {
		return "", false, nil
	}

	if gameConf.isActive() && gameConf.hostID != playerID {
		return "", false, errors.New("the player is not a host of this game")
	}

	pack, err := g.wdb.GetWordPack(langID, packID)
	if err != nil {
		return "", false, err
	}

	gameConf.pack = pack

	if gameConf.isActive() {
		g.setWord(gameConf)
	}

	g.games.Set(chatID, gameConf, g.exp)

	return gameConf.word, gameConf.hasDefinition(), nil
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

	return true
}

func (g *Game) CheckGuess(chatID, playerID int64, guess string) (string, string, bool) {
	gameConf, ok := g.games.Get(chatID)
	if !ok {
		return "", "", false
	}

	if !gameConf.checkGuess(playerID, guess) {
		return "", "", false
	}

	word := gameConf.word
	def := gameConf.def
	gameConf.setNotActive()

	return word, def, true
}

func (g *Game) NextWord(chatID, hostID int64) (string, bool, bool) {
	gameConf, ok := g.games.Get(chatID)
	if !ok {
		return "", false, false
	}

	if gameConf.isActive() {
		return "", false, false
	}

	gameConf.hostID = hostID
	g.setWord(gameConf)

	return gameConf.word, gameConf.hasDefinition(), true
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
