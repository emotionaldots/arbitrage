// Author: EmotionalDots @ PTH
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
	"github.com/emotionaldots/whatapi"
)

type API struct {
	*whatapi.WhatAPI
	Source string
}

func (w *API) Do(typ string, id int) (resp *arbitrage.Response, err error) {
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

func (w *API) Download(id int, suffix string) error {
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

func ResponseToInfo(resp whatapi.Torrent) arbitrage.InfoRelease {
	r := arbitrage.InfoRelease{}
	r.Name = resp.Group.Name
	r.TorrentId = resp.Torrent.ID
	r.FilePath = resp.Torrent.FilePath
	r.Tags = resp.Group.Tags
	r.Description = resp.Group.WikiBody
	r.Image = resp.Group.WikiImage

	r.Format = resp.Torrent.Media + " / " + resp.Torrent.Format
	if resp.Torrent.HasLog {
		r.Format += " / " + strconv.Itoa(resp.Torrent.LogScore)
	}

	if resp.Torrent.Remastered {
		r.Year = resp.Torrent.RemasterYear
		r.RecordLabel = resp.Torrent.RemasterRecordLabel
		r.CatalogueNumber = resp.Torrent.RemasterCatalogueNumber
		r.Edition = resp.Torrent.RemasterTitle
	} else {
		r.Year = resp.Group.Year
		r.RecordLabel = resp.Group.RecordLabel
		r.CatalogueNumber = resp.Group.CatalogueNumber
		r.Edition = "Original Release"
	}

	for _, a := range resp.Group.MusicInfo.Composers {
		r.Composers = append(r.Composers, a.Name)
	}
	for _, a := range resp.Group.MusicInfo.Artists {
		r.Artists = append(r.Artists, a.Name)
	}
	for _, a := range resp.Group.MusicInfo.With {
		r.With = append(r.With, a.Name)
	}
	for _, a := range resp.Group.MusicInfo.DJ {
		r.DJ = append(r.DJ, a.Name)
	}
	for _, a := range resp.Group.MusicInfo.RemixedBy {
		r.RemixedBy = append(r.RemixedBy, a.Name)
	}
	for _, a := range resp.Group.MusicInfo.Producer {
		r.Producer = append(r.Producer, a.Name)
	}

	return r
}
