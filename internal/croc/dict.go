package croc

import (
	"fmt"
	"github.com/ianlewis/go-stardict"
	"github.com/ianlewis/go-stardict/dict"
	"log"
	"regexp"
	"strings"
)

type Dict struct {
	sds map[string]*stardict.Stardict

	linkRe  *regexp.Regexp
	htmlEsc *strings.Replacer
}

func NewDict() *Dict {
	return &Dict{
		sds:    make(map[string]*stardict.Stardict),
		linkRe: regexp.MustCompile(`https?://[\w\-./?#]+:?`),
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
}

func (d *Dict) LoadDict(lang, path string) error {
	sd, err := stardict.Open(path)
	if err != nil {
		return err
	}

	d.sds[lang] = sd
	return nil
}

func (d *Dict) FindDefinition(lang, query string, re *regexp.Regexp) (string, error) {
	sd, exists := d.sds[lang]
	if !exists {
		return "", fmt.Errorf("dictionary for language '%s' does not exist", lang)
	}

	entries, err := sd.Search(query)
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
				log.Println(def)
				if re != nil {
					match := re.FindStringSubmatch(def)
					if len(match) > 1 {
						def = match[1]
					}
				}
				def = strings.TrimSpace(def)
				def = d.linkRe.ReplaceAllString(def, "")
				def = d.htmlEsc.Replace(def)
				return def, nil
			default:
			}
		}
	}

	return "", fmt.Errorf("definition of word '%s' not found", query)
}
