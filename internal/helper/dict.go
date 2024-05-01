package helper

import (
	"bufio"
	"encoding/json"
	"fmt"
	bolt "go.etcd.io/bbolt"
	"log"
	"os"
	"strings"
)

type wordDef struct {
	Word     string
	Pos      string
	LangCode string `json:"lang_code"`
	Title    string
	Redirect string
	Senses   []struct {
		Glosses    []string
		RawGlosses []string `json:"raw_glosses"`
	}
}

func UpdateDictionary() {
	cfg, err := LoadConfig()
	if err != nil {
		panic(err)
	}

	db, err := bolt.Open(cfg.DictPath, 0600, nil)
	if err != nil {
		panic(err)
	}
	defer func(db *bolt.DB) {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}(db)

	for _, lang := range cfg.Languages {
		lu, err := newLangUpdater(db, lang)
		if err != nil {
			log.Println(err)
			continue
		}

		for _, pack := range lang.WordPacks {
			err := lu.updateWordPack(pack)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

type langUpdater struct {
	db      *bolt.DB
	langID  string
	allDefs map[string]*wordDef
}

func newLangUpdater(db *bolt.DB, lang LanguageConfig) (*langUpdater, error) {
	allDefs := make(map[string]*wordDef)

	fmt.Printf("Loading dictionary from file %s...\n", lang.Dict.Path)
	file, err := os.Open(lang.Dict.Path)
	if err != nil {
		return nil, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Println(err)
		}
	}(file)

	var wordCount int
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		defJson := scanner.Text()
		wd := &wordDef{}
		err = json.Unmarshal([]byte(defJson), wd)
		if err != nil {
			log.Println(err)
			continue
		}

		if wd.LangCode != "" && wd.LangCode != lang.ID {
			continue
		}

		if len(wd.Senses) == 0 && wd.Word != "" {
			continue
		}

		var key string
		if wd.Word != "" {
			if lang.Dict.Parts {
				key = wd.Pos + "/" + wd.Word
			} else {
				key = wd.Word
			}
		} else {
			key = "redirect/" + wd.Title
		}

		prevDef, exist := allDefs[key]
		if exist {
			prevDef.Senses = append(prevDef.Senses, wd.Senses...)
			continue
		}

		allDefs[key] = wd
		allDefs["low/"+strings.ToLower(key)] = wd
		if wd.Word != "" && lang.Dict.Parts {
			key = "any-pos/" + wd.Word
			allDefs[key] = wd
			allDefs["low/"+strings.ToLower(key)] = wd
		}
		wordCount++
	}

	if err := scanner.Err(); err != nil {
		log.Println(err)
	}

	if wordCount == 0 {
		return nil, fmt.Errorf("Dictionary in %s is empty\n", lang.Dict.Path)
	}

	fmt.Printf("Loaded %d words.\n", wordCount)

	lu := langUpdater{
		db:      db,
		langID:  lang.ID,
		allDefs: allDefs,
	}

	return &lu, err
}

func (lu langUpdater) updateWordPack(pack WordPackConfig) error {
	fmt.Printf("Updating word pack %s/%s...\n", lu.langID, pack.ID)

	file, err := os.Open(pack.Path)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Println(err)
		}
	}(file)

	words := make([]string, 0, 200)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		word := strings.TrimSpace(scanner.Text())
		if word != "" {
			words = append(words, word)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if len(words) == 0 {
		return fmt.Errorf("word pack in %s is empty", pack.Path)
	}

	tx, err := lu.db.Begin(true)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	bkt, err := tx.CreateBucketIfNotExists([]byte(lu.langID))
	if err != nil {
		return err
	}

	if pack.Part != "" {
		bkt, err = bkt.CreateBucketIfNotExists([]byte(pack.Part))
		if err != nil {
			return err
		}
	}

	var updated, notFound int

	for _, word := range words {
		def, err := lu.findDefinition(word, pack.Part)
		if err != nil {
			log.Println(err)
			notFound++
			continue
		}

		err = bkt.Put([]byte(word), []byte(def))
		if err != nil {
			log.Println(err)
			continue
		}

		updated++
	}

	fmt.Printf("Updated: %d. Not found: %d. Total: %d.\n",
		updated, notFound, len(words))

	return tx.Commit()
}

func (lu langUpdater) findDefinition(query, pos string) (string, error) {
	var key string
	if pos == "" {
		key = query
	} else {
		key = pos + "/" + query
	}
	wd, found := lu.allDefs[key]
	if !found {
		wd, found = lu.allDefs["low/"+strings.ToLower(key)]
	}
	if !found {
		key = "redirect/" + query
		wd, found = lu.allDefs[key]
		if !found {
			wd, found = lu.allDefs["low/"+strings.ToLower(key)]
		}
		if found {
			if pos == "" {
				key = wd.Redirect
			} else {
				key = pos + "/" + wd.Redirect
			}
			wd, found = lu.allDefs[key]
		}
	}
	if !found && pos != "" {
		key = "any-pos/" + query
		wd, found = lu.allDefs[key]
		if !found {
			wd, found = lu.allDefs["low/"+strings.ToLower(key)]
		}
	}
	if !found {
		return "", fmt.Errorf("definition of word '%s' not found", query)
	}

	var def string
	if len(wd.Senses) == 1 {
		if len(wd.Senses[0].RawGlosses) > 0 {
			def = wd.Senses[0].RawGlosses[0]
		} else if len(wd.Senses[0].Glosses) > 0 {
			def = wd.Senses[0].Glosses[0]
		} else {
			return "", fmt.Errorf("no glosses exist for word %s", query)
		}
	} else {
		for i := range wd.Senses {
			var sense string
			if len(wd.Senses[i].RawGlosses) > 0 {
				sense = wd.Senses[i].RawGlosses[0]
			} else if len(wd.Senses[i].Glosses) > 0 {
				sense = wd.Senses[i].Glosses[0]
			}
			if sense != "" {
				def += fmt.Sprintf("%d) %s\n", i+1, sense)
			}
		}
	}

	return def, nil
}
