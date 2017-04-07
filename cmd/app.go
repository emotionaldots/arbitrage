// Author: EmotionalDots @ PTH
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"flag"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/emotionaldots/arbitrage/pkg/api/gazelle"
	"github.com/emotionaldots/arbitrage/pkg/api/waffles"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
	"github.com/emotionaldots/arbitrage/pkg/model"
	"github.com/jinzhu/gorm"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

type Source struct {
	Url      string `toml:"url"`
	User     string `toml:"user"`
	Password string `toml:"password"`
}

type Config struct {
	DatabaseType string            `tom:"database_type"`
	Database     string            `toml:"database"`
	Responses    string            `toml:"responses"`
	Sources      map[string]Source `toml:"sources"`
}

type App struct {
	ConfigDir  string
	Config     Config
	DB         *gorm.DB
	Indexes    map[string]*gorm.DB
	ApiClients map[string]API
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

func (app *App) Init() {
	flag.Parse()
	app.ApiClients = make(map[string]API)
	app.Indexes = make(map[string]*gorm.DB)

	app.ConfigDir = os.Getenv("HOME") + "/.config/arbitrage"
	app.Config.DatabaseType = "sqlite3"
	app.Config.Database = app.ConfigDir
	app.Config.Sources = make(map[string]Source)

	f, err := os.Open(app.ConfigDir + "/config.toml")
	if os.IsNotExist(err) {
		app.Config.Sources["example"] = Source{Url: "https://example.com"}
		must(os.MkdirAll(app.ConfigDir, 0755))
		f, err = os.Create(app.ConfigDir + "/config.toml")
		must(err)
		must(toml.NewEncoder(f).Encode(app.Config))
	} else {
		must(err)
		_, err = toml.DecodeReader(f, &app.Config)
		must(err)
	}
	f.Close()
}

func ParseSourceId(source string) (string, int) {
	parts := strings.SplitN(source, ":", 2)
	if len(parts) != 2 {
		must(errors.New("Expected format 'source:id', not '" + source + "'"))
	}
	id, err := strconv.Atoi(parts[1])
	must(err)
	return parts[0], id
}

func (app *App) APIForSource(source string) API {
	if c, ok := app.ApiClients[source]; ok {
		return c
	}

	parts := strings.SplitN(source, ":", 2)
	source = parts[0]

	s := app.Config.Sources[source]
	if s.Url == "" {
		must(errors.New("Source '" + source + "' not found in config!"))
	}
	s.Url = strings.TrimSuffix(s.Url, "/") + "/"

	var c API
	if source == "wfl" {
		w, err := waffles.NewAPI(s.Url, "arbitrage/2017-03-26 - EmotionalDots@PTH")
		must(err)
		c = &WafflesAPI{w, source}

	} else {
		w, err := gazelle.NewAPI(s.Url, "arbitrage/2017-02-26 - EmotionalDots@PTH")
		must(err)
		c = &GazelleAPI{w, source}
	}

	app.ApiClients[source] = c
	return c
}

func (app *App) DoLogin(source string) {
	c := app.APIForSource(source)
	s := app.Config.Sources[source]
	log.Printf("[%s] Logging into %s as %s", source, s.Url, s.User)
	must(c.Login(s.User, s.Password))
}
