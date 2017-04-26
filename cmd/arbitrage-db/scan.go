package main

import (
	"flag"
	"log"
	"time"

	"github.com/emotionaldots/arbitrage/cmd"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
)

func (app *App) Scan() {
	responses := make(chan *arbitrage.Response, 10)

	for i := 1; i < flag.NArg(); i++ {
		go app.ScanTracker(flag.Arg(i), responses)
	}

	for resp := range responses {
		gt, err := app.IndexResponse(*resp)
		if err != nil {
			log.Printf("[%v] err: %v", resp, err)
			continue
		}
		log.Printf("[%v] ok: %v", resp, gt)
	}
}

func (app *App) ScanTracker(sourceId string, responses chan *arbitrage.Response) {
	source, id := cmd.ParseSourceId(sourceId)
	c := app.DoLogin(source)
	retries, maxId := 0, 0

	backoff := 2 * time.Second
	if source == "wfl" {
		backoff = 500 * time.Millisecond
	}

	for {
		time.Sleep(backoff)

		resp, err := c.Do("torrent", id)
		if err != nil {
			log.Printf("[%s:%d] %s", source, id, err)

			if err.Error() == "Request failed: bad id parameter" || err.Error() == "Parsing failed: no filelist found" {
				if id > maxId {
					time.Sleep(backoff)
					if _, err := c.Do("torrent", id+200); err == nil {
						maxId = id + 200
					}
				}
				if id < maxId {
					id++
					retries = 0
					continue
				}
			}

			retries++
			if retries > 3 {
				id++
				retries = 0
			} else {
				time.Sleep(5 * time.Duration(retries) * time.Minute)
			}
			continue
		}

		responses <- resp
		id++
		retries = 0
	}
}
