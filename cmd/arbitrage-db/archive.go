package main

import (
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/boltdb/bolt"
	"github.com/emotionaldots/arbitrage/cmd"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
)

// Opens a BoltDB archive for the given source.
// The BoltDB archives contain all crawled responses.
func (app *App) OpenBolt(source string) *bolt.DB {
	if _, ok := app.Config.Sources[source]; !ok {
		log.Fatal("Unknown source:", source)
	}
	if db, ok := app.Archives[source]; ok {
		return db
	}

	db, err := bolt.Open(app.ConfigDir+"/"+source+".bolt", 0600, nil)
	must(err)
	app.Archives[source] = db
	return db
}

// The "list" command simply lists all responses in the archive by their key,
// given a tracker source and a release type ("torrent", "group", "collage")
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

// The "fetch" command returns a raw response from the archive, given a
// tracker source, release type and ID
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

func (app *App) ArchiveResponse(resp arbitrage.Response) error {
	db := app.OpenBolt(resp.Source)

	var body bytes.Buffer
	w := gzip.NewWriter(&body)
	_, err := w.Write([]byte(resp.Response))
	must(err)
	must(w.Close())

	must(db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(resp.Type))
		if err != nil {
			return err
		}
		return b.Put([]byte(resp.UID()), body.Bytes())
	}))
	return nil
}

// boltFetchLatest returns the last crawled response given a key prefix
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
