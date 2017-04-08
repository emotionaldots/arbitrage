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
	"strings"

	"github.com/boltdb/bolt"
	"github.com/emotionaldots/arbitrage/cmd"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
	"github.com/emotionaldots/arbitrage/pkg/model"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
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
	app.Run()
}

type App struct {
	cmd.App
	Databases map[string]*bolt.DB
	Indexes   map[string]*gorm.DB
}

func (app *App) Run() {
	app.Databases = make(map[string]*bolt.DB)
	app.Indexes = make(map[string]*gorm.DB)
	app.HasDatabase = true
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
func (app *App) GetDatabaseForSource(source string) *gorm.DB {
	if db, ok := app.Indexes[source]; ok {
		return db
	}

	var db *gorm.DB
	var err error
	switch app.Config.DatabaseType {
	case "sqlite3":
		db, err = gorm.Open("sqlite3", app.Config.Database+"/"+source+".db")
		must(err)
		db.DB().SetMaxOpenConns(1)
	default:
		var cfg = strings.Replace(app.Config.Database, "arbitrage_db", "arbitrage_"+source, -1)
		db, err = gorm.Open(app.Config.DatabaseType, cfg)
	}
	must(err)
	app.Indexes[source] = db

	if source == "arbitrage" {
		inited := db.HasTable(arbitrage.Response{})
		must(db.AutoMigrate(&arbitrage.Release{}).Error)
		must(db.AutoMigrate(&arbitrage.Response{}).Error)
		if !inited {
			db.Model(arbitrage.Response{}).AddIndex("idx_source_id", "source", "type", "type_id")
		}
	} else {
		must(db.AutoMigrate(model.Torrent{}).Error)
		must(db.AutoMigrate(model.Group{}).Error)
	}

	return db
}

func (app *App) GetDatabase() *gorm.DB {
	return app.GetDatabaseForSource("arbitrage")
}
