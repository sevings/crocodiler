package croc

import (
	"fmt"
	bolt "go.etcd.io/bbolt"
	"log"
)

type Dict struct {
	db *bolt.DB
}

func NewDict(path string) (*Dict, error) {
	db, err := bolt.Open(path, 0400, &bolt.Options{ReadOnly: true})
	if err != nil {
		return nil, err
	}

	return &Dict{
		db: db,
	}, nil
}

func (d *Dict) FindDefinition(lang, part, query string) (string, error) {
	tx, err := d.db.Begin(false)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	bkt := tx.Bucket([]byte(lang))
	if bkt != nil {
		bkt = bkt.Bucket([]byte(part))
	}
	if bkt == nil {
		return "", fmt.Errorf("bucket '%s/%s' does not exist", lang, part)
	}

	def := bkt.Get([]byte(query))
	if def == nil {
		return "", fmt.Errorf("definition of word '%s' not found", query)
	}

	res := make([]byte, len(def))
	copy(res, def)

	return string(res), nil
}

func (d *Dict) Close() {
	err := d.db.Close()
	if err != nil {
		log.Println(err)
	}
}
