package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/didip/tollbooth"
	"github.com/emotionaldots/arbitrage/pkg/arbitrage"
	"github.com/emotionaldots/arbitrage/pkg/model"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// Command "serve" starts the HTTP API server, so clients can query
// for releases or browse the tracker archives.
func (app *App) Serve() {
	ipLookups := []string{"RemoteAddr"}

	// Rate limit: one request every 2 seconds allowed, with a burst rate of 5
	shortLim := tollbooth.NewLimiter(5, 2*time.Second, nil)
	shortLim.SetIPLookups(ipLookups)

	// Crawling limit: We only allow 10k requests in a span of 3 days
	longLim := tollbooth.NewLimiter(10000, 30*time.Second, nil)
	longLim.SetIPLookups(ipLookups)

	r := mux.NewRouter()
	for source := range app.Config.Sources {
		fmt.Println("http://localhost:8321/" + source + "/ajax.php")
		r.HandleFunc("/"+source+"/ajax.php", app.handleAjax)
	}

	// The batch API is a lot more limited: max. 500 requests in 10 days
	batchLim := tollbooth.NewLimiter(500, 30*time.Minute, nil)
	batchLim.SetIPLookups(ipLookups)
	fmt.Println("http://localhost:8321/api/query")
	fmt.Println("http://localhost:8321/api/query_batch")
	r.Handle("/api/query_batch", tollbooth.LimitFuncHandler(batchLim, app.handleApiQueryBatch))
	r.HandleFunc("/api/query", app.handleApiQuery)

	h := tollbooth.LimitHandler(longLim, r)
	// h = tollbooth.LimitHandler(shortLim, h)
	h = handlers.CombinedLoggingHandler(os.Stdout, h)
	h = handlers.ProxyHeaders(h)
	h = handlers.RecoveryHandler()(h)

	must(http.ListenAndServe("0.0.0.0:8321", h))
}

type AjaxResult struct {
	Status   string      `json:"status"`
	Response interface{} `json:"response"`
}

// handleAjax provides an API that is very similar to the original Gazelle
// one, serving torrents, groups and collages from the database for a single tracker source.
// It allows cross-referencing releases from other trackers with a special
// parameter "xref=[source]".
func (app *App) handleAjax(w http.ResponseWriter, r *http.Request) {
	source := strings.TrimLeft(path.Dir(r.URL.Path), "/")
	if _, ok := app.Config.Sources[source]; !ok {
		http.Error(w, "Unknown source: "+source, 400)
		return
	}

	var result interface{}
	var err error
	db := app.GetDatabaseForSource(source)

	action := r.FormValue("action")
	id, _ := strconv.Atoi(r.FormValue("id"))

	if xref := r.FormValue("xref"); xref != "" {
		id64, err := app.CrossReference(action, xref, int64(id), source)
		if err != nil {
			jsonError(w, err.Error(), 404)
			return
		}
		id = int(id64)
	}

	switch action {
	case "torrent":
		gt := model.TorrentAndGroup{}
		gt.Torrent.ID = id
		err = db.Where(gt.Torrent).First(&gt.Torrent).Error
		if err == nil {
			gt.Group.ID = gt.Torrent.GroupID
			err = db.Where(gt.Group).First(&gt.Group).Error
		}
		result = gt
	case "torrentgroup":
		gt := model.GroupAndTorrents{}
		gt.Group.ID = id
		err = db.Where(gt.Group).First(&gt.Group).Error
		if err == nil {
			err = db.Where(model.Torrent{GroupID: id}).Find(&gt.Torrents).Error
		}
		result = gt
	default:
		jsonError(w, "Unknown action:"+action, 400)
		return
	}

	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	res := AjaxResult{"success", result}
	raw, _ := json.Marshal(res)
	w.Write(raw)
}

func jsonError(w http.ResponseWriter, err string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	res := AjaxResult{"error", err}
	raw, _ := json.Marshal(res)
	w.Write(raw)
}

type minimalRelease struct {
	Id       int64  `json:"id"`
	Hash     string `json:"hash"`
	FilePath string `json:"filePath"`
}

// handleApiQueryBatch provides batch functionality for the hash-based
// lookup of the arbitrage client.
// The client submits a list of filelist hashes and we return a number
// of tracker IDs that match the given hashes.
func (app *App) handleApiQueryBatch(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	if r.Method != "POST" {
		http.Error(w, "Bad Request", 400)
		return
	}

	source := r.PostFormValue("source")
	if len(source) == 0 || len(source) > 10 {
		jsonError(w, "No source given", 400)
		return
	}

	hashes := r.PostForm["hashes"]
	if len(hashes) == 0 || len(hashes) > 100 {
		jsonError(w, "Invalid number of hashes given", 400)
		return
	}

	db := app.GetDatabase()
	var releases []*arbitrage.Release
	err := db.Where("hash IN (?)", hashes).Where(arbitrage.Release{
		Source: source,
	}).Find(&releases).Error
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	result := make([]minimalRelease, len(releases))
	for i, r := range releases {
		result[i] = minimalRelease{
			Id:   r.SourceId,
			Hash: r.Hash,
		}
	}

	resp := map[string]interface{}{"torrents": result}
	res := AjaxResult{"success", resp}
	raw, _ := json.Marshal(res)
	w.Write(raw)
}

// handleApiQuery provides a hash-based lookup for the arbitrage client.
// The client submits a filelist hash and we return a tracker ID that
// matches the release, if found.
func (app *App) handleApiQuery(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	source := r.FormValue("source")
	if len(source) == 0 || len(source) > 10 {
		jsonError(w, "No source given", 400)
		return
	}

	hash := r.FormValue("hash")
	if len(hash) == 0 {
		jsonError(w, "No hash given", 400)
		return
	}

	db := app.GetDatabase()
	var releases []*arbitrage.Release
	err := db.Where(arbitrage.Release{
		Hash:   hash,
		Source: source,
	}).Find(&releases).Error
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	result := make([]minimalRelease, len(releases))
	for i, r := range releases {
		result[i] = minimalRelease{
			Id:       r.SourceId,
			Hash:     r.Hash,
			FilePath: r.FilePath,
		}
	}

	resp := map[string]interface{}{"torrents": result}
	res := AjaxResult{"success", resp}
	raw, _ := json.Marshal(res)
	w.Write(raw)
}
