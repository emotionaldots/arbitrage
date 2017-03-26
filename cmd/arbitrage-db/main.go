// Author: EmotionalDots @ PTH
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/emotionaldots/arbitrage/cmd"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
)

const Usage = `Usage: arbitrage [command] [args...]

Tracker API commands:
	scan [source:id...]:       Fetch torrents from trackers, starting at id
	scantorrent [source:id]:   Fetch a single torrent from tracker
	scancollages [source]:   Fetch a single collage from tracker
	recalculate:               Recalculate all hashes from saved API responses
`

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func dbSource(r *arbitrage.Release) arbitrage.Release {
	return arbitrage.Release{SourceId: r.SourceId}
}

func main() {
	app := App{}
	app.Run()
}

type App struct {
	cmd.App
}

func (app *App) Run() {
	app.Init()

	switch flag.Arg(0) {
	case "scan":
		app.Scan()
	case "scantorrent":
		app.ScanTorrent()
	case "recalculate":
		app.Recalculate()
	case "serve":
		app.Serve()
	case "import":
		app.Import()
	default:
		fmt.Println(Usage)
	}
}

func (app *App) ScanTorrent() {
	source, id := cmd.ParseSourceId(flag.Arg(1))

	db := app.GetDatabase()
	c := app.APIForSource(source)

	resp, err := c.Do("torrent", id)
	must(err)
	must(db.Create(resp).Error)

	r, err := c.FromResponse(resp)
	must(err)
	must(db.Where(dbSource(r)).Assign(r).FirstOrCreate(r).Error)

	fmt.Println(r.ListHash)
	fmt.Println(r.NameHash)
	fmt.Println(r.SizeHash)
}

func (app *App) Scan() {
	db := app.GetDatabase()

	responses := make(chan *arbitrage.Response, 10)

	for i := 1; i < flag.NArg(); i++ {
		go app.ScanTracker(flag.Arg(i), responses)
	}

	for resp := range responses {
		must(db.Create(resp).Error)
		c := app.APIForSource(resp.Source)

		r, err := c.FromResponse(resp)
		if err != nil {
			log.Printf("[%s:%d] %s", resp.Source, resp.TypeId, err)
			continue
		}

		must(db.Where(dbSource(r)).Assign(r).FirstOrCreate(r).Error)
		fmt.Println(r.SourceId, r.ListHash, r.FilePath)
	}
}

func (app *App) ScanTracker(sourceId string, responses chan *arbitrage.Response) {
	source, id := cmd.ParseSourceId(sourceId)
	c := app.APIForSource(source)
	retries, maxId := 0, 0

	backoff := 2 * time.Second
	if source == "wfl" {
		backoff = 500 * time.Millisecond
	}

	for {
		time.Sleep(backoff)

		resp, err := c.Do("torrent", id)
		if err != nil {
			log.Printf("[%s:%d] %s", source, id, err)

			if err.Error() == "Request failed: bad id parameter" {
				if id > maxId {
					time.Sleep(backoff)
					if _, err := c.Do("torrent", id+200); err == nil {
						maxId = id + 200
					}
				}
				if id < maxId {
					id++
					retries = 0
					continue
				}
			}

			retries++
			if retries > 3 {
				id++
				retries = 0
			} else {
				time.Sleep(5 * time.Duration(retries) * time.Minute)
			}
			continue
		}

		responses <- resp
		id++
		retries = 0
	}
}

func (app *App) Recalculate() {
	db := app.GetDatabase()

	offset := 0
	for {
		rs := make([]*arbitrage.Response, 0)
		db.Order("id asc").Limit(1000).Offset(offset).Find(&rs)
		offset += len(rs)
		if len(rs) == 0 {
			break
		}

		tx := db.Begin()
		for _, resp := range rs {
			c := app.APIForSource(resp.Source)
			r, err := c.FromResponse(resp)
			if err != nil {
				log.Printf("[%s:%d] %s", resp.Source, resp.TypeId, err)
				continue
			}
			must(tx.Where(dbSource(r)).Assign(r).FirstOrCreate(r).Error)
		}
		must(tx.Commit().Error)
		fmt.Println(offset)
	}
}

func (app *App) Serve() {
	for source := range app.Config.Sources {
		fmt.Println("http://localhost:8080/" + source + "/ajax.php")
		http.HandleFunc("/"+source+"/ajax.php", app.handleAjax)
	}
	must(http.ListenAndServe("0.0.0.0:8080", nil))
}

type AjaxResult struct {
	Status   string      `json:"status"`
	Response interface{} `json:"response"`
}

func (app *App) handleAjax(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	source := strings.TrimLeft(path.Dir(r.URL.Path), "/")
	if _, ok := app.Config.Sources[source]; !ok {
		http.Error(w, "Unknown source: "+source, 400)
		return
	}

	var result interface{}
	db := app.GetDatabase()

	action := r.FormValue("action")
	switch action {
	case "torrent":
		id, _ := strconv.Atoi(r.FormValue("id"))
		resp := &arbitrage.Response{
			Source: source,
			Type:   action,
			TypeId: id,
		}
		db.Where(resp).Last(resp)
		json.Unmarshal([]byte(resp.Response), &result)
	default:
		http.Error(w, "Unknown action: "+action, 400)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	res := AjaxResult{"success", result}
	raw, _ := json.MarshalIndent(res, "", "    ")
	w.Write(raw)
}

var reId = regexp.MustCompile(`_(\d+)_what_cd.json$`)

func (r response) checkStatus() error {
	if r.Status != "success" {
		return errors.New("API response error: " + r.Error)
	}
	return nil
}

type response struct {
	Status   string           `json:"status"`
	Error    string           `json:"error"`
	Response *json.RawMessage `json:"response"`
}

func (app *App) Import() {
	source := flag.Arg(1)
	file := flag.Arg(2)

	db := app.GetDatabase()

	z, err := zip.OpenReader(file)
	must(err)

	responses := make(chan *arbitrage.Response, 10)
	toStore := make(chan interface{}, 10)

	go func() {
		for i, f := range z.File {
			fmt.Println(i, f.Name)
			match := reId.FindStringSubmatch(f.Name)
			if match == nil {
				log.Println("Could not find ID in file name, skipping!")
				continue
			}
			id, err := strconv.Atoi(match[1])
			must(err)

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

			resp := &arbitrage.Response{
				Source:   source,
				Type:     "torrentgroup",
				TypeId:   id,
				Time:     time.Now(),
				Response: string(*rawResp.Response),
			}
			responses <- resp
			rc.Close()
		}
	}()

	go func() {
		for resp := range responses {
			toStore <- resp
			if err := app.InsertTorrentGroup(resp, toStore); err != nil {
				log.Println(err)
				continue
			}
		}
	}()

	for v := range toStore {
		switch r := v.(type) {
		case *arbitrage.Response:
			must(db.Create(r).Error)
		case *arbitrage.Release:
			must(db.Where(dbSource(r)).Assign(r).FirstOrCreate(r).Error)
			fmt.Println(r.SourceId, r.ListHash, r.FilePath)
		}
	}
}

type torrentGroup struct {
	Group    *json.RawMessage   `json:"group"`
	Torrents []*json.RawMessage `json:"torrents"`
}
type torrentSingle struct {
	Group   *json.RawMessage `json:"group"`
	Torrent *json.RawMessage `json:"torrent"`
}
type torrentPartial struct {
	Id int `json:"id"`
}

func (app *App) InsertTorrentGroup(resp *arbitrage.Response, toStore chan interface{}) error {
	c := app.APIForSource(resp.Source)
	var group torrentGroup
	if err := json.Unmarshal([]byte(resp.Response), &group); err != nil {
		return err
	}

	if len(group.Torrents) == 0 {
		return errors.New("No torrents found")
	}
	for _, t := range group.Torrents {
		var partial torrentPartial
		if err := json.Unmarshal([]byte(*t), &partial); err != nil {
			return err
		}
		if partial.Id == 0 {
			return errors.New("Missing ID")
		}
		ts := torrentSingle{group.Group, t}
		r2 := &arbitrage.Response{
			Source: resp.Source,
			Type:   "torrent",
			TypeId: partial.Id,
			Time:   resp.Time,
		}
		raw, err := json.Marshal(ts)
		if err != nil {
			return err
		}
		r2.Response = string(raw)
		toStore <- r2

		r, err := c.FromResponse(r2)
		must(err)
		toStore <- r
	}

	return nil
}
