package main

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"regexp"
	"strconv"

	"github.com/boltdb/bolt"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
)

var reId = regexp.MustCompile(`_(\d+)_what_cd.json$`)

type response struct {
	Status   string           `json:"status"`
	Error    string           `json:"error"`
	Response *json.RawMessage `json:"response"`
}

func (r response) checkStatus() error {
	if r.Status != "success" {
		return errors.New("API response error: " + r.Error)
	}
	return nil
}

// Command "import" imports all metadata responses from the "wcdjson.zip"
// archive into our BoltDB archive.
func (app *App) Import() {
	source := flag.Arg(1)

	z, err := zip.OpenReader(flag.Arg(2))
	must(err)

	must(err)
	resps := make(chan []arbitrage.Response)

	go func() {
		buf := make([]arbitrage.Response, 1000)
		var i int
		var f *zip.File
		for i, f = range z.File {
			fmt.Println(i, f.Name)
			match := reId.FindStringSubmatch(f.Name)
			if match == nil {
				log.Println("Could not find ID in file name, skipping!")
				continue
			}
			id, err := strconv.Atoi(match[1])
			if err != nil {
				must(errors.New("Invalid ID: " + match[1]))
			}

			rc, err := f.Open()
			must(err)

			var rawResp response
			err = json.NewDecoder(rc).Decode(&rawResp)
			if err == nil {
				err = rawResp.checkStatus()
			}
			if err != nil {
				log.Println(err)
				continue
			}
			must(rc.Close())

			resp := arbitrage.Response{
				Source:     source,
				Type:       "torrentgroup",
				TypeId:     id,
				Identifier: f.Name,
				Response:   string(*rawResp.Response),
				Time:       f.FileHeader.ModTime(),
			}

			if i > 0 && i%1000 == 0 {
				resps <- buf
				buf = make([]arbitrage.Response, 1000)
				buf[0] = resp
			} else {
				buf[i%1000] = resp
			}
		}
		resps <- buf[0:(i % 1000)]
		close(resps)
	}()

	db := app.OpenBolt(source)
	for rs := range resps {
		must(db.Update(func(tx *bolt.Tx) error {
			b, err := tx.CreateBucketIfNotExists([]byte("torrentgroup"))
			must(err)
			b.FillPercent = 1

			for _, resp := range rs {
				var body bytes.Buffer
				w := gzip.NewWriter(&body)
				_, err := w.Write([]byte(resp.Response))
				must(err)
				must(w.Close())

				b.Put([]byte(resp.UID()), body.Bytes())
			}

			return nil
		}))
	}
}
