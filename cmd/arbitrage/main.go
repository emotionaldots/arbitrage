// Author: EmotionalDots @ PTH
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/emotionaldots/arbitrage/cmd"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
	"github.com/jinzhu/gorm"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

var bootstrapUrl string

const Usage = `Usage: arbitrage [command] [args...]

Local directory commands:
	lookup [dir]:  Find releases with matching hash for directory
	hash   [dir]:  Print hashes for a torrent directory
	import [dir]:  Import torrent directory in database

Database commands:
	bootstrap:       Downloads a pre-populated database.
	info [hash|id]:  Print release information for a specific hash or id
	dump:            Dump the whole database

Tracker API commands:
	download [source:id]                Download a torrent from tracker

Example Usage:
	arbitrage lookup "./Various Artists - The What CD [FLAC]/"
	arbitrage download pth:41950
`

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
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
	case "hash":
		app.Hash()
	case "show":
		app.Info()
	case "info":
		app.Info()
	case "import":
		app.Import()
	case "lookup":
		app.Lookup()
	case "dump":
		app.Dump()
	case "download":
		app.Download()
	case "bootstrap":
		app.Bootstrap()
	default:
		fmt.Println(Usage)
	}
}

func (app *App) Hash() {
	dir := flag.Arg(1)
	r, err := arbitrage.FromFile(dir)
	must(err)

	fmt.Println(r.ListHash)
	// fmt.Println(r.NameHash)
	// fmt.Println(r.SizeHash)
}

func (app *App) Info() {
	arg := flag.Arg(1)
	db := app.GetDatabase()

	releases := make([]*arbitrage.Release, 0)
	must(db.Where(&arbitrage.Release{ListHash: arg}).
		Or(&arbitrage.Release{SourceId: arg}).
		Or(&arbitrage.Release{FilePath: arg}).
		Find(&releases).
		Error)

	for _, r := range releases {
		fmt.Printf("\n[%d] %s | %s\n%s\n=======================\n", r.Id, r.SourceId, r.ListHash, r.FilePath)
		files := arbitrage.ParseFileList(r.FileList)
		for _, f := range files {
			fmt.Printf("%s [%d]\n", f.Name, f.Size)
		}
	}
}

func (app *App) GetDatabase() *gorm.DB {
	db, err := gorm.Open("sqlite3", app.Config.Database)
	must(err)
	app.DB = db

	must(db.AutoMigrate(&arbitrage.Release{}).Error)

	return db
}

func dbSource(r *arbitrage.Release) arbitrage.Release {
	return arbitrage.Release{SourceId: r.SourceId}
}

func (app *App) Import() {
	dir := flag.Arg(1)
	r, err := arbitrage.FromFile(dir)
	must(err)

	db := app.GetDatabase()
	must(db.Where(dbSource(r)).Assign(r).FirstOrCreate(r).Error)

	fmt.Println(r.ListHash)
	// fmt.Println(r.NameHash)
	// fmt.Println(r.SizeHash)
}

func (app *App) Lookup() {
	dir := flag.Arg(1)
	r, err := arbitrage.FromFile(dir)
	must(err)

	db := app.GetDatabase()

	releases := make([]*arbitrage.Release, 0)
	must(db.Where(&arbitrage.Release{
		ListHash: r.ListHash,
	}).Find(&releases).Error)

	for _, other := range releases {
		state := "ok"
		if r.FilePath != other.FilePath {
			state = "renamed"
		}
		fmt.Println(state, other.SourceId, `"`+other.FilePath+`"`)
	}
}

func (app *App) Dump() {
	db := app.GetDatabase()
	rows, err := db.Model(&arbitrage.Release{}).
		Order("list_hash").
		Rows()
	must(err)
	defer rows.Close()

	for rows.Next() {
		r := &arbitrage.Release{}
		db.ScanRows(rows, &r)
		fmt.Println(r.ListHash, r.SourceId, `"`+r.FilePath+`"`)
	}
}

func (app *App) Download() {
	source, id := cmd.ParseSourceId(flag.Arg(1))
	c := app.APIForSource(source)

	u, err := c.CreateDownloadURL(id)
	must(err)

	resp, err := http.Get(u)
	must(err)
	if resp.StatusCode != 200 {
		must(errors.New("unexpected status: " + resp.Status))
	}
	defer resp.Body.Close()

	f, err := os.Create(source + "-" + strconv.Itoa(id) + ".torrent")
	defer f.Close()
	must(err)
	_, err = io.Copy(f, resp.Body)
	must(err)
}

func (app *App) Bootstrap() {
	if bootstrapUrl == "" {
		log.Fatal("No download URL included in this build, please download manually and place in " + app.Config.Database)
	}
	f, err := os.OpenFile(app.Config.Database, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	if os.IsExist(err) {
		log.Fatal("Database file " + app.Config.Database + " already exists, aborting boostrap.")
	}
	must(err)

	defer f.Close()

	resp, err := http.Get(bootstrapUrl)
	must(err)
	defer resp.Body.Close()

	gz, err := gzip.NewReader(resp.Body)
	must(err)

	fmt.Println("Downloading and unpacking database into " + app.Config.Database + ". This may take a few minutes")
	n, err := io.Copy(f, gz)
	must(err)
	fmt.Printf("%d MiB downloaded.", n/1024/1024)
}
