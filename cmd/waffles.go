// Author: EmotionalDots @ PTH
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"html"
	"net/url"
	"strconv"
	"time"

	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
	"github.com/emotionaldots/arbitrage/pkg/waffles"
)

type WafflesAPI struct {
	*waffles.API
	Source string
}

func (w *WafflesAPI) Do(typ string, id int) (resp *arbitrage.Response, err error) {
	resp = &arbitrage.Response{
		Source: w.Source,
		Type:   typ,
		TypeId: id,
		Time:   time.Now(),
	}

	var raw []byte
	switch typ {
	case "torrent":
		raw, err = w.DoTorrent(id, url.Values{})
	default:
		return nil, errors.New("Unknown type: " + typ)
	}
	if err != nil {
		return resp, err
	}

	resp.Response = string(raw)
	return resp, nil
}

func (w *WafflesAPI) FromResponse(resp *arbitrage.Response) (*arbitrage.Release, error) {
	r := &arbitrage.Release{}
	if resp.Type != "torrent" {
		return r, errors.New("Expected response of type 'torrent' not: " + resp.Type)
	}
	r.SourceId = resp.Source + ":" + strconv.Itoa(resp.TypeId)

	t, err := w.ParseTorrent([]byte(resp.Response))
	if err != nil {
		return nil, err
	}

	r.FileList = t.Torrent.FileList
	r.FilePath = t.Torrent.FilePath
	fmt.Println(t.Group.Name)

	files := arbitrage.ParseFileList(html.UnescapeString(r.FileList))
	r.FileList = arbitrage.FilesToList(files)
	r.FilePath = html.UnescapeString(r.FilePath)
	r.CalculateHashes()
	return r, nil
}

func (w *WafflesAPI) Download(id int, suffix string) error {
	return nil
}

func (w *WafflesAPI) ResponseToInfo(resp *arbitrage.Response) arbitrage.InfoRelease {
	t, err := w.ParseTorrent([]byte(resp.Response))
	must(err)

	r := arbitrage.InfoRelease{}
	r.Name = t.Group.Name
	r.TorrentId = t.Torrent.ID
	r.FilePath = t.Torrent.FilePath
	r.Tags = t.Group.Tags
	r.Description = t.Group.WikiBody
	r.Image = t.Group.WikiImage

	r.Format = t.Torrent.Media + " / " + t.Torrent.Format
	if t.Torrent.HasLog {
		r.Format += " / " + strconv.Itoa(t.Torrent.LogScore)
	}

	if t.Torrent.Remastered {
		r.Year = t.Torrent.RemasterYear
		r.RecordLabel = t.Torrent.RemasterRecordLabel
		r.CatalogueNumber = t.Torrent.RemasterCatalogueNumber
		r.Edition = t.Torrent.RemasterTitle
	} else {
		r.Year = t.Group.Year
		r.RecordLabel = t.Group.RecordLabel
		r.CatalogueNumber = t.Group.CatalogueNumber
		r.Edition = "Original Release"
	}

	for _, a := range t.Group.MusicInfo.Composers {
		r.Composers = append(r.Composers, a.Name)
	}
	for _, a := range t.Group.MusicInfo.Artists {
		r.Artists = append(r.Artists, a.Name)
	}
	for _, a := range t.Group.MusicInfo.With {
		r.With = append(r.With, a.Name)
	}
	for _, a := range t.Group.MusicInfo.DJ {
		r.DJ = append(r.DJ, a.Name)
	}
	for _, a := range t.Group.MusicInfo.RemixedBy {
		r.RemixedBy = append(r.RemixedBy, a.Name)
	}
	for _, a := range t.Group.MusicInfo.Producer {
		r.Producer = append(r.Producer, a.Name)
	}

	return r
}