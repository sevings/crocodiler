package croc

import (
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
)

type Dict struct {
	db  *bolt.DB
	log *zap.SugaredLogger
}

func NewDict(path string) (*Dict, bool) {
	log := zap.L().Named("dict").Sugar()
	db, err := bolt.Open(path, 0400, &bolt.Options{ReadOnly: true})
	if err != nil {
		log.Error(err)
		return nil, false
	}

	return &Dict{
		db:  db,
		log: log,
	}, true
}

func (d *Dict) FindDefinition(lang, part, query string) (string, bool) {
	tx, err := d.db.Begin(false)
	if err != nil {
		d.log.Error(err)
		return "", false
	}
	defer func() { _ = tx.Rollback() }()

	bkt := tx.Bucket([]byte(lang))
	if bkt != nil && part != "" {
		bkt = bkt.Bucket([]byte(part))
	}
	if bkt == nil {
		d.log.Errorw("bucket does not exist",
			"lang", lang,
			"part", part)
		return "", false
	}

	def := bkt.Get([]byte(query))
	if def == nil {
		d.log.Warnw("definition of the word not found",
			"query", query)
		return "", false
	}

	res := make([]byte, len(def))
	copy(res, def)

	return string(res), true
}

func (d *Dict) Close() {
	err := d.db.Close()
	if err != nil {
		d.log.Warn(err)
	}
}
