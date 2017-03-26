// Author: EmotionalDots @ PTH
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
	"github.com/emotionaldots/whatapi"
)

type API interface {
	Login(username, password string) error
	Do(typ string, id int) (resp *arbitrage.Response, err error)
	Download(id int, suffix string) error
	FromResponse(resp *arbitrage.Response) (*arbitrage.Release, error)
	ResponseToInfo(resp *arbitrage.Response) arbitrage.InfoRelease
}

type GazelleAPI struct {
	*whatapi.WhatAPI
	Source string
}

func (w *GazelleAPI) Do(typ string, id int) (resp *arbitrage.Response, err error) {
	resp = &arbitrage.Response{
		Source: w.Source,
		Type:   typ,
		TypeId: id,
		Time:   time.Now(),
	}

	var result interface{}
	switch typ {
	case "torrent":
		result, err = w.GetTorrent(id, url.Values{})
	default:
		return nil, errors.New("Unknown type: " + typ)
	}
	if err != nil {
		return resp, err
	}

	raw, err := json.Marshal(result)
	if err != nil {
		return resp, err
	}
	resp.Response = string(raw)
	return resp, nil
}

func (w *GazelleAPI) Download(id int, suffix string) error {
	u, err := w.CreateDownloadURL(id)
	if err != nil {
		return err
	}

	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.New("unexpected status: " + resp.Status)
	}
	defer resp.Body.Close()

	f, err := os.Create(w.Source + "-" + strconv.Itoa(id) + suffix + ".torrent")
	defer f.Close()
	if err != nil {
		return err
	}

	_, err = io.Copy(f, resp.Body)
	return err
}

func (w *GazelleAPI) FromResponse(resp *arbitrage.Response) (*arbitrage.Release, error) {
	r := &arbitrage.Release{}
	if resp.Type != "torrent" {
		return r, errors.New("Expected response of type 'torrent' not: " + resp.Type)
	}
	r.SourceId = resp.Source + ":" + strconv.Itoa(resp.TypeId)

	raw := json.RawMessage(resp.Response)
	t := whatapi.Torrent{}
	if err := json.Unmarshal(raw, &t); err != nil {
		return r, err
	}
	r.FileList = t.Torrent.FileList
	r.FilePath = t.Torrent.FilePath

	files := arbitrage.ParseFileList(html.UnescapeString(r.FileList))
	r.FileList = arbitrage.FilesToList(files)
	r.FilePath = html.UnescapeString(r.FilePath)
	r.CalculateHashes()
	return r, nil
}

func (w *GazelleAPI) ResponseToInfo(resp *arbitrage.Response) arbitrage.InfoRelease {
	raw := json.RawMessage(resp.Response)
	t := whatapi.Torrent{}
	must(json.Unmarshal(raw, &t))

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
