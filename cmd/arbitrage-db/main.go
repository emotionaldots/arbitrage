// Author: EmotionalDots @ PTH
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"flag"
	"fmt"
	"log"

	"github.com/boltdb/bolt"
	"github.com/emotionaldots/arbitrage/cmd"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
)

const Usage = `Usage: arbitrage [command] [args...]

Tracker API commands:
	scan [source:id...]:       Fetch torrents from trackers, starting at id
	scancollages [source]:     Fetch a single collage from tracker
	recalculate:               Recalculate all hashes from saved API responses
`

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func dbSource(r arbitrage.Release) arbitrage.Release {
	return arbitrage.Release{
		Source:   r.Source,
		SourceId: r.SourceId,
		HashType: r.HashType,
	}
}

func main() {
	app := App{}
	app.Databases = make(map[string]*bolt.DB)
	app.Run()
}

type App struct {
	cmd.App
	Databases map[string]*bolt.DB
}

func (app *App) Run() {
	app.Init()

	switch flag.Arg(0) {
	case "scan":
		app.Scan()
	case "recalculate":
		app.Recalculate()
	case "serve":
		app.Serve()
	case "import":
		app.Import()
	case "convert":
		app.Convert()
	case "list":
		app.List()
	case "fetch":
		app.Fetch()
	default:
		fmt.Println(Usage)
	}
}

func (app *App) CrossReference(typ string, source string, id int64, target string) (int64, error) {
	if typ != "torrent" {
		return 0, errors.New("Type " + typ + " not implemented")
	}

	db := app.GetDatabase()

	src := arbitrage.Release{
		Source:   source,
		SourceId: id,
	}
	if err := db.Where(src).First(&src).Error; err != nil {
		return 0, err
	}

	dest := arbitrage.Release{
		Source: target,
		Hash:   src.Hash,
	}
	if err := db.Where(dest).First(&dest).Error; err != nil {
		return 0, err
	}

	return dest.SourceId, nil
}
