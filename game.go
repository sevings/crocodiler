package main

import (
	"errors"
	"github.com/erni27/imcache"
	"strings"
	"time"
)

type gameConfig struct {
	pack   WordPack
	word   string
	hostID int64
}

func (gc *gameConfig) isActive() bool {
	return gc.word != ""
}

func (gc *gameConfig) setNotActive() {
	gc.word = ""
}

func (gc *gameConfig) setWord() {
	gc.word = gc.pack.GetWord()
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
}

func NewGame(db *DB, wdb *WordDB) *Game {
	return &Game{
		db:  db,
		wdb: wdb,
	}
}

func (g *Game) Play(chatID, hostID int64) (string, error) {
	chatConf := g.db.LoadChatConfig(chatID)
	if chatConf.PackID == "" {
		return "", errors.New("word pack is not chosen")
	}

	pack, err := g.wdb.GetWordPack(chatConf.LangID, chatConf.PackID)
	if err != nil {
		return "", err
	}

	gameConf := gameConfig{
		pack:   pack,
		hostID: hostID,
	}

	gameConf.setWord()

	g.games.Set(chatID, &gameConf, imcache.WithSlidingExpiration(24*time.Hour))

	return gameConf.word, nil
}

func (g *Game) SetWordPack(chatID, playerID int64, langID, packID string) (string, error) {
	gameConf, ok := g.games.Get(chatID)
	if !ok {
		return "", nil
	}

	if gameConf.isActive() && gameConf.hostID != playerID {
		return "", errors.New("the player is not a host of this game")
	}

	pack, err := g.wdb.GetWordPack(langID, packID)
	if err != nil {
		return "", err
	}

	gameConf.pack = pack

	if gameConf.isActive() {
		gameConf.setWord()
	}

	g.games.Set(chatID, gameConf, imcache.WithSlidingExpiration(24*time.Hour))

	return gameConf.word, nil
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

	g.games.Set(chatID, gameConf, imcache.WithSlidingExpiration(24*time.Hour))

	return true
}

func (g *Game) CheckGuess(chatID, playerID int64, guess string) (string, bool) {
	gameConf, ok := g.games.Get(chatID)
	if !ok {
		return "", false
	}

	if !gameConf.checkGuess(playerID, guess) {
		return "", false
	}

	word := gameConf.word
	gameConf.setNotActive()

	return word, true
}

func (g *Game) NextWord(chatID, hostID int64) (string, bool) {
	gameConf, ok := g.games.Get(chatID)
	if !ok {
		return "", false
	}

	if gameConf.isActive() {
		return "", false
	}

	gameConf.hostID = hostID
	gameConf.setWord()

	return gameConf.word, true
}

func (g *Game) SkipWord(chatID, playerID int64) (string, bool) {
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

	gameConf.setWord()

	return gameConf.word, true
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
