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
	"strings"

	"github.com/boltdb/bolt"
	"github.com/emotionaldots/arbitrage/cmd"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
	"github.com/emotionaldots/arbitrage/pkg/model"
	"github.com/jinzhu/gorm"
)

// UpdateIndex updates the SQL database for the current model (group or torrent)
func updateIndex(db *gorm.DB, m interface{}) error {
	id := 0
	switch v := m.(type) {
	case model.Torrent:
		log.Printf("  - %v", v)
		id = v.ID
		if id == 0 {
			return errors.New("Indexer: no ID found")
		}
		return db.Where("id = ?", id).Assign(v).FirstOrCreate(&v).Error
	case model.Group:
		log.Printf("  - %v", v)
		id = int(v.ID)
		if id == 0 {
			return errors.New("Indexer: no ID found")
		}
		return db.Where("id = ?", id).Assign(v).FirstOrCreate(&v).Error
	case model.GroupAndTorrents:
		if err := updateIndex(db, v.Group); err != nil {
			return err
		}
		for _, t := range v.Torrents {
			if err := updateIndex(db, t); err != nil {
				return err
			}
		}
	case model.CollageWithGroups:
		id = v.ID
		log.Printf("  - %v", v)
		for _, g := range v.TorrentGroups {
			ct := model.CollagesTorrents{CollageID: id, GroupID: int(g.ID)}
			if err := db.Where(ct).Assign(ct).FirstOrCreate(&ct).Error; err != nil {
				return err
			}
		}
		c := v.Collage
		return db.Where("id = ?", id).Assign(c).FirstOrCreate(&c).Error
	default:
		return fmt.Errorf("update: unsupported type %T", m)
	}
	return nil
}

// Command "recalculate" iterates over all crawled response in the BoltDB archive,
// parses them and then rebuilds the torrent database.
func (app *App) Recalculate() {
	typ := flag.Arg(1)
	source := flag.Arg(2)
	id := 0
	if strings.Contains(flag.Arg(2), ":") {
		source, id = cmd.ParseSourceId(flag.Arg(2))
	}

	archive := app.OpenBolt(source)
	must(archive.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(typ))
		c := b.Cursor()
		k, v := c.First()
		if id > 0 {
			k, v = c.Seek([]byte(arbitrage.Pad(id)))
		}

		for k != nil {
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
			if err := app.IndexResponse(resp); err != nil {
				log.Printf("[%v] err: %s", resp, err.Error())
			}
			k, v = c.Next()
		}
		return nil
	}))
}

// IndexResponse parses an API response and stores the torrents and groups
// in the SQL database
func (app *App) IndexResponse(resp arbitrage.Response) error {
	api := app.APIForSource(resp.Source)
	idx := app.GetDatabaseForSource(resp.Source)
	db := app.GetDatabase()

	m, err := api.ParseResponseReleases(resp)
	if err != nil {
		return err
	}
	log.Printf("%v response parsed:", resp)

	if resp.Type == "collage" {
		if err := updateIndex(idx, m); err != nil {
			return err
		}
	}

	gs, err := model.NormalizeTorrentGroups(m)
	if err != nil {
		return err
	}
	for _, gt := range gs {
		if err := updateIndex(idx, gt); err != nil {
			return err
		}

		// Now we iterate over all torrents and calculate hashes for our
		// hash-based query API and store them in the database
		for _, t := range gt.Torrents {
			if t.FileList == "" {
				continue
			}
			r := arbitrage.Release{
				Source:   resp.Source,
				SourceId: int64(t.ID),
				FileList: arbitrage.ParseFileList(html.UnescapeString(t.FileList)),
				FilePath: html.UnescapeString(t.FilePath),
			}

			arbitrage.HashDefault(&r)
			must(db.Where(dbSource(r)).Assign(r).FirstOrCreate(&r).Error)

			log.Printf("  - hash: %v", r.FilePath)
		}
	}

	return nil
}
