package helper

import (
	"bufio"
	"fmt"
	"github.com/ianlewis/go-stardict"
	"github.com/ianlewis/go-stardict/dict"
	bolt "go.etcd.io/bbolt"
	"log"
	"os"
	"regexp"
	"strings"
)

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
		sd, err := stardict.Open(lang.Dict.Path)
		if err != nil {
			log.Println(err)
		}

		lu := langUpdater{
			db:      db,
			sd:      sd,
			langID:  lang.ID,
			pattern: lang.Dict.Pattern,
			linkRe:  regexp.MustCompile(`https?://[\w\-./?#]+:?`),
			htmlEsc: strings.NewReplacer(
				"&lt;", "<",
				"&gt;", ">",
				"&#34;", "\"",
				"&#39;", "'",
				"<br>", "\n",
				"", "\r",
				"&amp;", "&",
			),
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
	sd      *stardict.Stardict
	langID  string
	pattern string

	linkRe  *regexp.Regexp
	htmlEsc *strings.Replacer
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

	pattern := fmt.Sprintf(lu.pattern, pack.Part)
	defRe, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

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

	bkt, err = bkt.CreateBucketIfNotExists([]byte(pack.Part))
	if err != nil {
		return err
	}

	var updated, notFound int

	for _, word := range words {
		def, err := lu.findDefinition(word, defRe)
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

func (lu langUpdater) findDefinition(query string, re *regexp.Regexp) (string, error) {
	entries, err := lu.sd.Search(query)
	if err != nil {
		return "", err
	}

	var sde *stardict.Entry
	for _, entry := range entries {
		if entry.Title() == query {
			sde = entry
			break
		}
	}

	if sde == nil {
		query = strings.ToLower(query)
		for _, entry := range entries {
			if strings.ToLower(entry.Title()) == query {
				sde = entry
				break
			}
		}
	}

	if sde != nil {
		for _, data := range sde.Data() {
			switch data.Type {
			case dict.UTFTextType:
				def := string(data.Data)
				if re != nil {
					match := re.FindStringSubmatch(def)
					if len(match) > 1 {
						def = match[1]
					}
				}
				def = strings.TrimSpace(def)
				def = lu.linkRe.ReplaceAllString(def, "")
				def = lu.htmlEsc.Replace(def)
				return def, nil
			default:
			}
		}
	}

	return "", fmt.Errorf("definition of word '%s' not found", query)
}
