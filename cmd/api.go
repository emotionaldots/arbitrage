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

	"github.com/emotionaldots/arbitrage/pkg/api/gazelle"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
	"github.com/emotionaldots/arbitrage/pkg/model"
)

type API interface {
	Login(username, password string) error
	Do(typ string, id int) (resp *arbitrage.Response, err error)
	Download(id int, suffix string) error
	ParseResponseReleases(resp arbitrage.Response) (model.GroupAndTorrents, error)
}

type GazelleAPI struct {
	*gazelle.API
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

func (w *GazelleAPI) ParseResponseReleases(resp arbitrage.Response) (model.GroupAndTorrents, error) {
	g := model.GroupAndTorrents{}

	var result interface{}
	switch resp.Type {
	case "torrent":
		gt := model.TorrentAndGroup{}
		if err := json.Unmarshal([]byte(resp.Response), &gt); err != nil {
			return g, err
		}
		result = gt
	case "torrentgroup":
		gt := model.GroupAndTorrents{}
		if err := json.Unmarshal([]byte(resp.Response), &gt); err != nil {
			return g, err
		}
		result = gt
	default:
		return g, errors.New("API: unexpected response type: " + resp.Type)
	}
	return model.NormalizeTorrentGroups(result)
}
