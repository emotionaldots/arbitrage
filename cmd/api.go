// Author: EmotionalDots @ PTH
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/emotionaldots/arbitrage/pkg/api/gazelle"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
	"github.com/emotionaldots/arbitrage/pkg/model"
)

type API interface {
	Login(username, password string) error
	Do(typ string, id int) (resp *arbitrage.Response, err error)
	Download(id int) ([]byte, error)
	ParseResponseReleases(resp arbitrage.Response) (interface{}, error)
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
	case "collage":
		result, err = w.GetCollage(id, url.Values{})
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

func (w *GazelleAPI) Download(id int) ([]byte, error) {
	u, err := w.CreateDownloadURL(id)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, errors.New("unexpected status: " + resp.Status)
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func (w *GazelleAPI) ParseResponseReleases(resp arbitrage.Response) (interface{}, error) {
	var result interface{}
	switch resp.Type {
	case "torrent":
		gt := model.TorrentAndGroup{}
		if err := json.Unmarshal([]byte(resp.Response), &gt); err != nil {
			return nil, err
		}
		result = gt
	case "torrentgroup":
		gt := model.GroupAndTorrents{}
		if err := json.Unmarshal([]byte(resp.Response), &gt); err != nil {
			return nil, err
		}
		result = gt
	case "collage":
		c := model.CollageWithGroups{}
		if err := json.Unmarshal([]byte(resp.Response), &c); err != nil {
			return nil, err
		}
		result = c
	default:
		return nil, errors.New("API: unexpected response type: " + resp.Type)
	}
	return result, nil
}
