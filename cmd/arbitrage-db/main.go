// Author: EmotionalDots @ PTH
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"path"
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

	r, err := arbitrage.FromResponse(resp)
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

		r, err := arbitrage.FromResponse(resp)
		must(err)

		must(db.Where(dbSource(r)).Assign(r).FirstOrCreate(r).Error)
		fmt.Println(r.SourceId, r.ListHash, r.FilePath)
	}
}

func (app *App) ScanTracker(sourceId string, responses chan *arbitrage.Response) {
	source, id := cmd.ParseSourceId(sourceId)
	c := app.APIForSource(source)
	retries, maxId := 0, 0

	for {
		time.Sleep(500 * time.Millisecond)

		resp, err := c.Do("torrent", id)
		if err != nil {
			log.Printf("[%s:%d] %s", source, id, err)

			if err.Error() == "Request failed: bad id parameter" {
				if id > maxId {
					time.Sleep(500 * time.Millisecond)
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
			r, err := arbitrage.FromResponse(resp)
			must(err)
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
