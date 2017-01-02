// Author: EmotionalDots @ PTH
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/emotionaldots/arbitrage/cmd"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

var bootstrapUrl string

const Usage = `Usage: arbitrage [command] [args...]

Local directory commands:
	lookup [dir]:  Find releases with matching hash for directory
	hash   [dir]:  Print hashes for a torrent directory
	import [dir]:  Import torrent directory into database

Database commands:
	bootstrap:              Downloads a pre-populated database.
	info [hash|source:id]:  Print release information for a specific hash or id
	dump:                   Dump the whole database

Tracker API commands:
	download [source:id]          Download a torrent from tracker
	downthemall [source] [dirs]:  Walk through all subdirectories and download matching torrents
	tag [dir]                     Download metatadata into file
	tagthemall [dirs]:            Walk through all subdirectories and download metadata

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
	case "downthemall":
		app.DownThemAll()
	case "bootstrap":
		app.Bootstrap()
	case "tag":
		app.Tag()
	case "tagthemall":
		app.TagThemAll()
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
	must(c.Download(id, ""))
}

func (app *App) DownThemAll() {
	source := flag.Arg(1)
	dir := flag.Arg(2)
	fdir, err := os.Open(dir)
	must(err)
	defer fdir.Close()

	names, err := fdir.Readdirnames(-1)
	must(err)

	c := app.APIForSource(source)

	logf, err := os.Create("arbitrage.log")
	must(err)
	defer logf.Close()
	lw := io.MultiWriter(os.Stdout, logf)
	fmt.Fprintf(logf, "#!/usr/bin/env bash\n## arbitrage downthemall %s %q\n\n\n", source, dir)

	db := app.GetDatabase()

	for _, n := range names {
		r, err := arbitrage.FromFile(dir + "/" + n)
		must(err)

		releases := make([]*arbitrage.Release, 0)
		must(db.Where(&arbitrage.Release{
			ListHash: r.ListHash,
		}).Find(&releases).Error)

		for _, other := range releases {
			s, id := cmd.ParseSourceId(other.SourceId)
			if s != source {
				continue
			}

			status := "ok"
			if r.FilePath != other.FilePath {
				status = "renamed"
			}

			if err := c.Download(id, "-"+status); err != nil {
				log.Printf("[%s] Could not download torrent: %s\n", other.SourceId, err)
				continue
			}

			if status == "renamed" {
				fmt.Fprintf(lw, "mv %q %q    # %s\n", r.FilePath, other.FilePath, other.SourceId)
			} else {
				fmt.Fprintf(lw, "# ok %s %q\n", other.SourceId, other.FilePath)
			}
			time.Sleep(200 * time.Millisecond) // Rate-limiting
			break
		}
	}
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

func (app *App) Tag() {
	dir := flag.Arg(1)
	sourceId := flag.Arg(2)
	app.tagSingle(dir, sourceId)
}

func (app *App) TagThemAll() {
	dir := flag.Arg(1)

	fdir, err := os.Open(dir)
	must(err)
	defer fdir.Close()

	names, err := fdir.Readdirnames(-1)
	must(err)

	for _, name := range names {
		app.tagSingle(dir+"/"+name, "")
		time.Sleep(200 * time.Millisecond)
	}
}

func (app *App) tagSingle(dir, sourceId string) {
	r, err := arbitrage.FromFile(dir)
	db := app.GetDatabase()
	must(err)

	sourceIds := make([]string, 0)
	if sourceId == "" {
		releases := make([]*arbitrage.Release, 0)
		must(db.Where(&arbitrage.Release{
			ListHash: r.ListHash,
		}).Find(&releases).Error)

		for _, other := range releases {
			sourceIds = append(sourceIds, other.SourceId)
		}
	} else {
		sourceIds = append(sourceIds, sourceId)
	}

	if len(sourceIds) == 0 {
		return
	}

	info := &arbitrage.Info{
		Version:     1,
		FileHash:    r.ListHash,
		LastUpdated: time.Now(),
		Releases:    make(map[string]arbitrage.InfoRelease),
	}

	for _, sourceId := range sourceIds {
		source, id := cmd.ParseSourceId(sourceId)
		if _, ok := app.Config.Sources[source]; !ok {
			continue
		}

		c := app.APIForSource(source)
		t, err := c.GetTorrent(id, url.Values{})
		if err != nil {
			log.Printf("[%s] error - %s (%s)", sourceId, err, r.FilePath)
			continue
		}

		info.Releases[source] = cmd.ResponseToInfo(t)
		log.Printf("[%s] %s", sourceId, info.Releases[source])
	}
	if len(info.Releases) == 0 {
		return
	}

	f, err := os.Create(dir + "/release.info.yaml")
	must(err)

	y, err := yaml.Marshal(info)
	must(err)

	_, err = f.Write(y)
	must(err)

	defer f.Close()

}
