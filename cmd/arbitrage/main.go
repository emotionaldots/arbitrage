// Author: EmotionalDots @ PTH
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/emotionaldots/arbitrage/cmd"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage/torrentinfo"
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
	arbitrage.HashDefault(r)

	c := client.New(app.Config.Server, cmd.UserAgent)
	releases, err := c.Query(source, []string{r.Hash})
	must(err)

	for _, other := range releases {
		state := "ok"
		if other.FilePath == "" {
			state = "no_filepath"
		} else if r.FilePath != other.FilePath {
			state = "renamed"
		}
		fmt.Printf("%s %s:%d %q\n", state, source, other.Id, other.FilePath)
	}
}

func (app *App) Download() {
	source, id := cmd.ParseSourceId(flag.Arg(1))
	c := app.DoLogin(source)

	torrent, err := c.Download(id)
	must(err)

	name, err := app.GetTorrentName(torrent)
	must(err)
	log.Println(name)

	path := source + "-" + strconv.Itoa(id) + ".torrent"
	must(app.SaveTorrent(torrent, path))
}

func (app *App) GetTorrentName(torrent []byte) (string, error) {
	mi, err := torrentinfo.Load(bytes.NewReader(torrent))
	if err != nil {
		return "", err
	}
	i, err := mi.UnmarshalInfo()
	if err != nil {
		return "", err
	}
	return i.Name, nil
}

func (app *App) SaveTorrent(torrent []byte, path string) error {
	return ioutil.WriteFile(path, torrent, 0644)
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
			var releases []client.Release
			for i := 0; i < 3; i++ {
				if releases, err = c.Query(source, hashes); err == nil {
					break
				}
				log.Printf("error on try %d/3: %s", i, err)
				time.Sleep(5 * time.Second)
			}
			must(err)

			byHash := make(map[string][]client.Release, 0)
			for _, r := range releases {
				byHash[r.Hash] = append(byHash[r.Hash], r)
			}
			for i, job := range jobs {
				job.Releases = byHash[job.Hash]
				jobs[i] = job
			}

			queue <- jobs
			hashes = make([]string, 0, 100)
			jobs = make([]job, 0, 100)
		}

		for _, n := range names {
			r, err := arbitrage.FromFile(dir + "/" + n)
			must(err)
			arbitrage.HashDefault(r)

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

	for jobs := range app.batchQueryDirectory(dir, source) {
		for _, job := range jobs {
			for _, other := range job.Releases {
				torrent, err := c.Download(int(other.Id))
				if err != nil {
					log.Printf("[%s:%d] Could not download torrent, skipping: %s\n", source, other.Id, err)
					continue
				}

				path, err := app.GetTorrentName(torrent)
				if err != nil {
					log.Printf("[%s:%d] Invalid torrent file, skipping: %s\n", source, other.Id, err)
					continue
				}

				status := "ok"
				if job.LocalDir != path {
					status = "renamed"
					fmt.Fprintf(lw, "mv %q %q    # %s:%d\n", job.LocalDir, path, source, other.Id)
				} else {
					fmt.Fprintf(lw, "# ok %s:%d %q\n", source, other.Id, path)
				}

				tfile := fmt.Sprintf("%s-%d-%s.torrent", source, other.Id, status)
				must(app.SaveTorrent(torrent, tfile))

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
