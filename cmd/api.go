// Author: EmotionalDots @ PTH
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
	"github.com/kdvh/whatapi"
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
