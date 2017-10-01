package main

import (
	"flag"
	"log"
	"strings"
	"time"

	"github.com/emotionaldots/arbitrage/cmd"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
)

// Command "scan" sequentially requests torrents via API from multiple tracker sources,
// archives their responses and indexes them for the database.
// You can specify multiple sources, optionally with an ID used as a starting point, e.g. "scan apl red wcd" or "scan red:12345 wfl"
func (app *App) Scan() {
	responses := make(chan *arbitrage.Response, 10)

	for i := 1; i < flag.NArg(); i++ {
		go app.ScanTracker(flag.Arg(i), responses)
	}

	for resp := range responses {
		err := app.ArchiveResponse(*resp)
		must(err)
		err = app.IndexResponse(*resp)
		if err != nil {
			log.Printf("[%v] err: %v", resp, err)
			continue
		}
	}
}

// ScanTracker sequentially crawls API responses of a given type from a single
// tracker and returns the responses in an async channel.
// This function blocks indefinitely and needs to be manually killed to stop
// crawling.
// It has an backoff/retry algorithm to catch server errors or sleep for a few
// minutes if the end of sequential torrents was reached.
func (app *App) ScanTracker(sourceId string, responses chan *arbitrage.Response) {
	typ := "torrent"
	if strings.Count(sourceId, ":") > 1 {
		parts := strings.SplitN(sourceId, ":", 2)
		typ, sourceId = parts[0], parts[1]
	}
	source, id := cmd.ParseSourceId(sourceId)

	c := app.DoLogin(source)
	retries, maxId := 0, 0
	// minimum API request rate, as allowed per the rules
	backoff := 2 * time.Second
	// number of releases to skip forward to determine whether the current
	// release is simply no longer available (deleted) or we reached the end
	// of results
	lookAhead := 50
	if typ == "torrent" {
		lookAhead = 500
	}

	for {
		time.Sleep(backoff)

		resp, err := c.Do(typ, id)
		if err != nil {
			log.Printf("[%s:%d] %s", source, id, err)

			if err.Error() == "Request failed: bad id parameter" || err.Error() == "Parsing failed: no filelist found" {
				if id > maxId {
					time.Sleep(backoff)
					if _, err := c.Do(typ, id+lookAhead); err == nil {
						maxId = id + lookAhead
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
