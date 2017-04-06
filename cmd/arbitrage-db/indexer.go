package main

import (
	"bytes"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"html"
	"io/ioutil"
	"log"

	"github.com/boltdb/bolt"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
	"github.com/emotionaldots/arbitrage/pkg/model"
	"github.com/jinzhu/gorm"
)

func updateIndex(db *gorm.DB, m interface{}) error {
	id := 0
	switch v := m.(type) {
	case model.Torrent:
		id = v.ID
		if id == 0 {
			return errors.New("Indexer: no ID found")
		}
		return db.Where("id = ?", id).Assign(v).FirstOrCreate(&v).Error
	case model.Group:
		id = v.ID
		if id == 0 {
			return errors.New("Indexer: no ID found")
		}
		return db.Where("id = ?", id).Assign(v).FirstOrCreate(&v).Error
	case model.GroupAndTorrents:
		for _, t := range v.Torrents {
			if err := updateIndex(db, t); err != nil {
				return err
			}
		}
		return updateIndex(db, v.Group)
	}
	return fmt.Errorf("update: unsupported type %T", m)
}

func (app *App) Recalculate() {
	typ := flag.Arg(1)
	source := flag.Arg(2)

	archive := app.OpenBolt(source)
	must(archive.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(typ))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			r, err := gzip.NewReader(bytes.NewReader(v))
			must(err)
			body, err := ioutil.ReadAll(r)
			must(err)

			resp := arbitrage.Response{
				Source:     source,
				Type:       typ,
				Identifier: string(k),
				Response:   string(body),
			}
			if _, err := app.IndexResponse(resp); err != nil {
				log.Printf("[%v] err: %s", resp, err.Error())
			}
		}
		return nil
	}))
}

func (app *App) IndexResponse(resp arbitrage.Response) (model.GroupAndTorrents, error) {
	api := app.APIForSource(resp.Source)
	idx := app.GetDatabaseForSource(resp.Source)
	db := app.GetDatabase()

	gt, err := api.ParseResponseReleases(resp)
	if err != nil {
		return gt, err
	}
	must(updateIndex(idx, gt))

	for _, t := range gt.Torrents {
		r := arbitrage.Release{
			Source:   resp.Source,
			SourceId: int64(t.ID),
			FileList: arbitrage.ParseFileList(html.UnescapeString(t.FileList)),
			FilePath: html.UnescapeString(t.FilePath),
		}

		arbitrage.HashDefault(&r)
		must(db.Where(dbSource(r)).Assign(r).FirstOrCreate(&r).Error)

		log.Printf("[%v] ok: %v", resp, gt)
	}

	return gt, nil
}
