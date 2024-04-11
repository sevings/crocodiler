package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strings"
)

type WordPack []string

func (pack WordPack) GetWord() string {
	i := rand.Intn(len(pack))
	return pack[i]
}

func loadWordPack(path string) (WordPack, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	pack := make([]string, 0, 200)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		word := strings.TrimSpace(scanner.Text())
		if word != "" {
			pack = append(pack, word)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(pack) == 0 {
		return nil, fmt.Errorf("word pack in %s is empty", path)
	}

	return pack, nil
}

type WordDB struct {
	langIDs   []string
	packIDs   map[string][]string
	langNames map[string]string
	packNames map[string]map[string]string
	packs     map[string]map[string]WordPack
}

func NewWordDB() *WordDB {
	return &WordDB{
		langIDs:   make([]string, 0),
		packIDs:   make(map[string][]string),
		langNames: make(map[string]string),
		packNames: make(map[string]map[string]string),
		packs:     make(map[string]map[string]WordPack),
	}
}

func (db *WordDB) GetLanguageIDs() []string {
	return db.langIDs
}

func (db *WordDB) GetWordPackIDs(langID string) ([]string, error) {
	names, ok := db.packIDs[langID]
	if !ok {
		return nil, fmt.Errorf("language %s does not exist", langID)
	}

	return names, nil
}

func (db *WordDB) GetLanguageName(langID string) (string, error) {
	name, ok := db.langNames[langID]
	if !ok {
		return "", fmt.Errorf("language %s does not exist", langID)
	}

	return name, nil
}

func (db *WordDB) GetWordPackName(langID, packID string) (string, error) {
	var name string
	names, ok := db.packNames[langID]
	if ok {
		name, ok = names[packID]
	}
	if !ok {
		return "", fmt.Errorf("word pack %s/%s does not exist", langID, packID)
	}

	return name, nil
}

func (db *WordDB) GetWordPack(langID, packID string) (WordPack, error) {
	pack, ok := db.packs[langID][packID]
	if !ok {
		return nil, fmt.Errorf("word pack %s/%s does not exist", langID, packID)
	}

	return pack, nil
}

func (db *WordDB) LoadWordPack(path, langID, packID, langName, packName string) error {
	pack, err := loadWordPack(path)
	if err != nil {
		return err
	}

	if _, ok := db.packs[langID]; !ok {
		db.langIDs = append(db.langIDs, langID)
		db.packIDs[langID] = make([]string, 0)
		db.langNames[langID] = langName
		db.packNames[langID] = make(map[string]string)
		db.packs[langID] = make(map[string]WordPack)
	}

	db.packs[langID][packID] = pack
	db.packIDs[langID] = append(db.packIDs[langID], packID)
	db.packNames[langID][packID] = packName

	return nil
}
