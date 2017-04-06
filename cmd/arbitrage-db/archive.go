package main

import (
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/emotionaldots/arbitrage/cmd"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
)

func (app *App) OpenBolt(source string) *bolt.DB {
	if _, ok := app.Config.Sources[source]; !ok {
		log.Fatal("Unknown source:", source)
	}
	if db, ok := app.Databases[source]; ok {
		return db
	}

	db, err := bolt.Open(app.ConfigDir+"/"+source+".bolt", 0600, nil)
	must(err)
	app.Databases[source] = db
	return db
}

func (app *App) Convert() {
	var num, i int64
	db := app.GetDatabase()
	db.Model(&arbitrage.Response{}).Count(&num)

	rows, err := db.Model(&arbitrage.Response{}).Rows()
	must(err)
	defer rows.Close()

	chans := make(map[string]chan *arbitrage.Response)
	fin := sync.WaitGroup{}

	for rows.Next() {
		resp := &arbitrage.Response{}
		db.ScanRows(rows, resp)
		i++
		if i%1000 == 0 {
			log.Printf("%d / %d (%.2f%%)", i, num, float64(i)/float64(num)*100)
		}

		ch, ok := chans[resp.Source]
		if ok {
			ch <- resp
			continue
		}

		ch = make(chan *arbitrage.Response, 100)
		chans[resp.Source] = ch
		ch <- resp

		fin.Add(1)
		go func(source string) {
			bname := []byte("torrent")
			if source == "wfl" {
				bname = []byte("torrent.html")
			}
			db := app.OpenBolt(source)
			tx, err := db.Begin(true)
			must(err)
			i := 1
			_, err = tx.CreateBucketIfNotExists(bname)
			must(err)
			b := tx.Bucket(bname)

			for resp := range ch {
				var body bytes.Buffer
				w := gzip.NewWriter(&body)
				_, err := w.Write([]byte(resp.Response))
				must(err)
				must(w.Close())

				b.Put([]byte(resp.UID()), body.Bytes())

				i++
				if i%100 == 0 {
					must(tx.Commit())
					tx, err = db.Begin(true)
					must(err)
					b = tx.Bucket(bname)
				}
			}

			must(tx.Commit())
			fin.Done()
		}(resp.Source)
	}

	for _, ch := range chans {
		close(ch)
	}
	fin.Wait()
}

func (app *App) List() {
	typ := flag.Arg(1)
	source := flag.Arg(2)

	db := app.OpenBolt(source)
	must(db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(typ))
		c := b.Cursor()

		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			fmt.Println(string(k))
		}
		return nil
	}))
}

func (app *App) Fetch() {
	typ := flag.Arg(1)
	source, id := cmd.ParseSourceId(flag.Arg(2))

	db := app.OpenBolt(source)
	must(db.View(func(tx *bolt.Tx) error {
		r, err := boltFetchLast(tx, typ, id)
		must(err)
		_, err = io.Copy(os.Stdout, r)
		must(err)
		return nil
	}))
}

func boltFetchLast(tx *bolt.Tx, typ string, id int) (io.Reader, error) {
	b := tx.Bucket([]byte(typ))

	c := b.Cursor()
	c.Seek([]byte(arbitrage.Pad(id + 1)))
	k, v := c.Prev()
	if k == nil || !bytes.HasPrefix(k, []byte(arbitrage.Pad(id))) {
		return nil, fmt.Errorf("bolt: %s %d not found", typ, id)
	}

	return gzip.NewReader(bytes.NewReader(v))
}
