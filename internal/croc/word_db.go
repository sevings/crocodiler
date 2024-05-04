package croc

import (
	"bufio"
	"go.uber.org/zap"
	"math/rand"
	"os"
	"strings"
)

type WordPack struct {
	langID string
	packID string
	part   string
	words  []string
}

func (pack *WordPack) GetWord() string {
	i := rand.Intn(len(pack.words))
	return pack.words[i]
}

func (pack *WordPack) GetLangID() string {
	return pack.langID
}

func (pack *WordPack) GetPackID() string {
	return pack.packID
}

func (pack *WordPack) GetPart() string {
	return pack.part
}

func (db *WordDB) loadWordPackImp(langID, packID, path, part string) (*WordPack, bool) {
	file, err := os.Open(path)
	if err != nil {
		db.log.Error(err)
		return nil, false
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			db.log.Warn(err)
		}
	}(file)

	pack := &WordPack{
		langID: langID,
		packID: packID,
		part:   part,
		words:  make([]string, 0, 200),
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		word := strings.TrimSpace(scanner.Text())
		if word != "" {
			pack.words = append(pack.words, word)
		}
	}

	if err := scanner.Err(); err != nil {
		db.log.Error(err)
		return nil, false
	}

	if len(pack.words) == 0 {
		db.log.Errorw("word pack is empty",
			"path", path)
		return nil, false
	}

	return pack, true
}

type WordDB struct {
	langIDs   []string
	packIDs   map[string][]string
	langNames map[string]string
	packNames map[string]map[string]string
	packs     map[string]map[string]*WordPack
	log       *zap.SugaredLogger
}

func NewWordDB() *WordDB {
	return &WordDB{
		langIDs:   make([]string, 0),
		packIDs:   make(map[string][]string),
		langNames: make(map[string]string),
		packNames: make(map[string]map[string]string),
		packs:     make(map[string]map[string]*WordPack),
		log:       zap.L().Named("word_db").Sugar(),
	}
}

func (db *WordDB) GetLanguageIDs() []string {
	return db.langIDs
}

func (db *WordDB) GetWordPackIDs(langID string) ([]string, bool) {
	names, ok := db.packIDs[langID]
	if !ok {
		db.log.Errorw("language doesn't exist", "lang_id", langID)
		return nil, false
	}

	return names, true
}

func (db *WordDB) GetLanguageName(langID string) (string, bool) {
	name, ok := db.langNames[langID]
	if !ok {
		db.log.Errorw("language doesn't exist", "lang_id", langID)
		return "", false
	}

	return name, true
}

func (db *WordDB) GetWordPackName(langID, packID string) (string, bool) {
	var name string
	names, ok := db.packNames[langID]
	if ok {
		name, ok = names[packID]
	}
	if !ok {
		db.log.Errorw("word pack doesn't exist",
			"lang_id", langID,
			"pack_id", packID)
		return "", false
	}

	return name, true
}

func (db *WordDB) GetWordPack(langID, packID string) (*WordPack, bool) {
	pack, ok := db.packs[langID][packID]
	if !ok {
		db.log.Errorw("word pack doesn't exist",
			"lang_id", langID,
			"pack_id", packID)
		return nil, false
	}

	return pack, true
}

func (db *WordDB) LoadWordPack(path, langID, packID, part, langName, packName string) bool {
	pack, ok := db.loadWordPackImp(langID, packID, path, part)
	if !ok {
		return false
	}

	if _, ok := db.packs[langID]; !ok {
		db.langIDs = append(db.langIDs, langID)
		db.packIDs[langID] = make([]string, 0)
		db.langNames[langID] = langName
		db.packNames[langID] = make(map[string]string)
		db.packs[langID] = make(map[string]*WordPack)
	}

	db.packs[langID][packID] = pack
	db.packIDs[langID] = append(db.packIDs[langID], packID)
	db.packNames[langID][packID] = packName

	return true
}
