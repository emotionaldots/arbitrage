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
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
	"github.com/emotionaldots/whatapi"
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
	Database  string            `toml:"database"`
	Responses string            `toml:"responses"`
	Sources   map[string]Source `toml:"sources"`
}

type App struct {
	Config     Config
	DB         *gorm.DB
	ApiClients map[string]*API
}

func (app *App) GetDatabase() *gorm.DB {
	if app.DB != nil {
		return app.DB
	}

	db, err := gorm.Open("sqlite3", app.Config.Database)
	must(err)
	app.DB = db

	inited := db.HasTable(arbitrage.Response{})
	must(db.AutoMigrate(&arbitrage.Release{}).Error)
	must(db.AutoMigrate(&arbitrage.Response{}).Error)
	if !inited {
		db.Model(arbitrage.Response{}).AddIndex("idx_source_id", "source", "type", "type_id")
	}

	return db
}

func (app *App) Init() {
	flag.Parse()
	app.ApiClients = make(map[string]*API)

	cfgDir := os.Getenv("HOME") + "/.config/arbitrage"
	app.Config.Sources = make(map[string]Source)
	app.Config.Database = cfgDir + "/arbitrage.db"

	f, err := os.Open(cfgDir + "/config.toml")
	if os.IsNotExist(err) {
		app.Config.Sources["example"] = Source{Url: "https://example.com"}
		must(os.MkdirAll(cfgDir, 0755))
		f, err = os.Create(cfgDir + "/config.toml")
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

func (app *App) APIForSource(source string) *API {
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

	w, err := whatapi.NewWhatAPI(s.Url)
	must(err)
	c := &API{w, source}
	must(c.Login(s.User, s.Password))

	app.ApiClients[source] = c
	return c
}
