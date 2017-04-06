// Author: EmotionalDots @ PTH
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/emotionaldots/arbitrage/cmd"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
	"github.com/emotionaldots/arbitrage/pkg/model"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

var bootstrapUrl string

const Usage = `Usage: arbitrage [command] [args...]

Local directory commands:
	lookup [dir]:  Find releases with matching hash for directory
	hash   [dir]:  Print hashes for a torrent directory
	import [dir]:  Import torrent directory into database

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
	case "lookup":
		app.Lookup()
	case "download":
		app.Download()
	case "downthemall":
		app.DownThemAll()
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
	arbitrage.HashDefault(r)
	fmt.Println(r.Hash)
}

func dbSource(r *arbitrage.Release) arbitrage.Release {
	return arbitrage.Release{SourceId: r.SourceId}
}

func (app *App) Lookup() {
	dir := flag.Arg(1)
	r, err := arbitrage.FromFile(dir)
	must(err)

	db := app.GetDatabase()

	releases := make([]*arbitrage.Release, 0)
	must(db.Where(&arbitrage.Release{
		Hash: r.Hash,
	}).Find(&releases).Error)

	for _, other := range releases {
		state := "ok"
		if r.FilePath != other.FilePath {
			state = "renamed"
		}
		fmt.Println(state, other.SourceId, `"`+other.FilePath+`"`)
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
			Hash:   r.Hash,
			Source: source,
		}).Find(&releases).Error)

		for _, other := range releases {
			status := "ok"
			if r.FilePath != other.FilePath {
				status = "renamed"
			}

			if err := c.Download(int(r.SourceId), "-"+status); err != nil {
				log.Printf("[%s:%d] Could not download torrent: %s\n", other.Source, other.SourceId, err)
				continue
			}

			if status == "renamed" {
				fmt.Fprintf(lw, "mv %q %q    # %s:%d\n", r.FilePath, other.FilePath, other.Source, other.SourceId)
			} else {
				fmt.Fprintf(lw, "# ok %s:%d %q\n", other.Source, other.SourceId, other.FilePath)
			}
			time.Sleep(200 * time.Millisecond) // Rate-limiting
			break
		}
	}
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
			Hash: r.Hash,
		}).Find(&releases).Error)

		for _, other := range releases {
			sourceIds = append(sourceIds, fmt.Sprintf("%s:%d", other.Source, other.SourceId))
		}
	} else {
		sourceIds = append(sourceIds, sourceId)
	}

	if len(sourceIds) == 0 {
		return
	}

	info := &arbitrage.Info{
		Version:     1,
		FileHash:    r.Hash,
		LastUpdated: time.Now(),
		Releases:    make(map[string]arbitrage.InfoRelease),
	}

	for _, sourceId := range sourceIds {
		source, id := cmd.ParseSourceId(sourceId)
		if _, ok := app.Config.Sources[source]; !ok {
			continue
		}

		c := app.APIForSource(source)
		resp, err := c.Do("torrent", id)
		if err != nil {
			log.Printf("[%s] error - %s (%s)", sourceId, err, r.FilePath)
			continue
		}
		gt, err := c.ParseResponseReleases(*resp)
		if err != nil {
			log.Printf("[%s] error - %s (%s)", sourceId, err, r.FilePath)
			continue
		}
		info.Releases[source] = GroupToInfo(gt)
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

func GroupToInfo(gt model.GroupAndTorrents) arbitrage.InfoRelease {
	g := gt.Group
	t := gt.Torrents[0]

	r := arbitrage.InfoRelease{}
	r.Name = g.Name
	r.TorrentId = t.ID
	r.FilePath = t.FilePath
	r.Tags = g.Tags
	r.Description = g.WikiBody
	r.Image = g.WikiImage

	r.Format = t.Media + " / " + t.Format
	if t.HasLog {
		r.Format += " / " + strconv.Itoa(t.LogScore)
	}

	if t.Remastered {
		r.Year = t.RemasterYear
		r.RecordLabel = t.RemasterRecordLabel
		r.CatalogueNumber = t.RemasterCatalogueNumber
		r.Edition = t.RemasterTitle
	} else {
		r.Year = g.Year
		r.RecordLabel = g.RecordLabel
		r.CatalogueNumber = g.CatalogueNumber
		r.Edition = "Original Release"
	}

	for _, a := range g.MusicInfo.Composers {
		r.Composers = append(r.Composers, a.Name)
	}
	for _, a := range g.MusicInfo.Artists {
		r.Artists = append(r.Artists, a.Name)
	}
	for _, a := range g.MusicInfo.With {
		r.With = append(r.With, a.Name)
	}
	for _, a := range g.MusicInfo.DJ {
		r.DJ = append(r.DJ, a.Name)
	}
	for _, a := range g.MusicInfo.RemixedBy {
		r.RemixedBy = append(r.RemixedBy, a.Name)
	}
	for _, a := range g.MusicInfo.Producer {
		r.Producer = append(r.Producer, a.Name)
	}

	return r
}
