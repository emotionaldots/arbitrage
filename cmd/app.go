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
	"github.com/shibukawa/configdir"
)

var UserAgent = "arbitrage/2017-04-08"

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
	Server       string            `toml:"server"`
	DatabaseType string            `toml:"database_type,omitempty"`
	Database     string            `toml:"database,omitempty"`
	Sources      map[string]Source `toml:"sources"`
}

type App struct {
	HasDatabase bool
	ConfigDir   string
	Config      Config
	ApiClients  map[string]API
}

func (app *App) Init() {
	flag.Parse()
	app.ApiClients = make(map[string]API)

	cfdir := configdir.New("", "arbitrage")
	app.ConfigDir = cfdir.QueryFolders(configdir.Global)[0].Path
	app.Config.Server = "https://arbitrage.invariant.space"
	app.Config.Sources = make(map[string]Source)

	if app.HasDatabase {
		app.Config.DatabaseType = "sqlite3"
		app.Config.Database = app.ConfigDir
	}

	f, err := os.Open(app.ConfigDir + "/config.toml")
	if os.IsNotExist(err) {
		app.Config.Sources["red"] = Source{Url: "https://redacted.ch"}
		app.Config.Sources["apl"] = Source{Url: "https://apollo.rip"}
		app.Config.Sources["wfl"] = Source{Url: "https://waffles.ch"}
		must(os.MkdirAll(app.ConfigDir, 0755))
		f, err = os.Create(app.ConfigDir + "/config.toml")
		must(err)
		must(toml.NewEncoder(f).Encode(app.Config))
		log.Println("Created new config in " + app.ConfigDir + "/config.toml")
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
		w, err := waffles.NewAPI(s.Url, UserAgent)
		must(err)
		c = &WafflesAPI{w, source}

	} else {
		w, err := gazelle.NewAPI(s.Url, UserAgent)
		must(err)
		c = &GazelleAPI{w, source}
	}

	app.ApiClients[source] = c
	return c
}

func (app *App) DoLogin(source string) API {
	c := app.APIForSource(source)
	s := app.Config.Sources[source]
	log.Printf("[%s] Logging into %s as %s", source, s.Url, s.User)
	must(c.Login(s.User, s.Password))
	return c
}
