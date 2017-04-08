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

	"github.com/emotionaldots/arbitrage/cmd"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
	"github.com/emotionaldots/arbitrage/pkg/client"
	"github.com/emotionaldots/arbitrage/pkg/model"
)

var bootstrapUrl string

const Usage = `Usage: arbitrage [command] [args...]

Local directory commands:
	lookup [source] [dir]: Find releases with matching hash for directory
	hash   [dir]:          Print hashes for a torrent directory

Tracker API commands:
	download [source:id]          Download a torrent from tracker
	downthemall [source] [dirs]:  Walk through all subdirectories and download matching torrents

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

func (app *App) Lookup() {
	source := flag.Arg(1)
	dir := flag.Arg(2)
	r, err := arbitrage.FromFile(dir)
	must(err)

	c := client.New(app.Config.Server, cmd.UserAgent)
	releases, err := c.Query(source, []string{dir})
	must(err)

	for _, other := range releases {
		state := "ok"
		if other.FilePath == "" {
			state = "no_filepath"
		} else if r.FilePath != other.FilePath {
			state = "renamed"
		}
		fmt.Printf("%s %s:%d %q\n", state, source, other.Id, `"`+other.FilePath+`"`)
	}
}

func (app *App) Download() {
	source, id := cmd.ParseSourceId(flag.Arg(1))
	c := app.DoLogin(source)
	must(c.Download(id, ""))
}

type job struct {
	LocalDir string
	Hash     string
	Releases []client.Release
}

func (app *App) batchQueryDirectory(dir, source string) chan []job {
	fdir, err := os.Open(dir)
	must(err)
	defer fdir.Close()

	names, err := fdir.Readdirnames(-1)
	must(err)

	queue := make(chan []job, 0)
	c := client.New(app.Config.Server, cmd.UserAgent)

	go func() {
		hashes := make([]string, 0, 100)
		jobs := make([]job, 0, 100)

		doQuery := func() {
			releases, err := c.Query(source, hashes)
			must(err)
			byHash := make(map[string][]client.Release, 0)
			for _, r := range releases {
				byHash[r.Hash] = append(byHash[r.Hash], r)
			}

			queue <- jobs
			hashes = make([]string, 0, 100)
			jobs = make([]job, 0, 100)
		}

		for _, n := range names {
			r, err := arbitrage.FromFile(dir + "/" + n)
			must(err)

			if len(jobs) >= 100 {
				doQuery()
			}
			jobs = append(jobs, job{r.FilePath, r.Hash, nil})
			hashes = append(hashes, r.Hash)
		}
		if len(jobs) > 0 {
			doQuery()
		}
		close(queue)
	}()
	return queue
}

func (app *App) DownThemAll() {
	source := flag.Arg(1)
	dir := flag.Arg(2)

	c := app.DoLogin(source)

	logf, err := os.Create("arbitrage.log")
	must(err)
	defer logf.Close()
	lw := io.MultiWriter(os.Stdout, logf)
	fmt.Fprintf(logf, "#!/usr/bin/env bash\n## arbitrage downthemall %s %q\n\n\n", source, dir)

	for jobs := range app.batchQueryDirectory(source, dir) {
		for _, job := range jobs {
			for _, other := range job.Releases {
				status := "ok"
				if other.FilePath == "" {
					status = "no_filepath"
				} else if job.LocalDir != other.FilePath {
					status = "renamed"
				}

				if err := c.Download(int(other.Id), "-"+status); err != nil {
					log.Printf("[%s:%d] Could not download torrent: %s\n", source, other.Id, err)
					continue
				}

				if status == "renamed" {
					fmt.Fprintf(lw, "mv %q %q    # %s:%d\n", job.LocalDir, other.FilePath, source, other.Id)
				} else {
					fmt.Fprintf(lw, "# ok %s:%d %q\n", source, other.Id, other.FilePath)
				}
				time.Sleep(200 * time.Millisecond) // Rate-limiting
				break
			}
		}
	}
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
