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

func (app *App) Serve() {
	shortLim := tollbooth.NewLimiter(5, 10*time.Second)
	shortLim.IPLookups = []string{"RemoteAddr"}

	longLim := tollbooth.NewLimiter(1000, 24*time.Hour)
	longLim.IPLookups = []string{"RemoteAddr"}

	r := mux.NewRouter()
	for source := range app.Config.Sources {
		fmt.Println("http://localhost:8321/" + source + "/ajax.php")
		r.HandleFunc("/"+source+"/ajax.php", app.handleAjax)
	}

	apiLim := tollbooth.NewLimiter(100, 24*time.Hour)
	apiLim.IPLookups = []string{"RemoteAddr"}
	fmt.Println("http://localhost:8321/api/query")
	r.Handle("/api/query", tollbooth.LimitFuncHandler(apiLim, app.handleApiQuery))

	h := tollbooth.LimitHandler(longLim, r)
	h = tollbooth.LimitHandler(shortLim, h)
	h = handlers.CombinedLoggingHandler(os.Stdout, h)
	h = handlers.ProxyHeaders(h)
	h = handlers.RecoveryHandler()(h)

	must(http.ListenAndServe("0.0.0.0:8321", h))
}

type AjaxResult struct {
	Status   string      `json:"status"`
	Response interface{} `json:"response"`
}

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

func (app *App) handleApiQuery(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	if r.Method != "POST" {
		http.Error(w, "Bad Request", 400)
		return
	}

	hashes := r.PostForm["hashes"]
	if len(hashes) == 0 || len(hashes) > 100 {
		jsonError(w, "Invalid number of hashes given", 400)
		return
	}

	source := r.PostForm.Get("source")
	if len(source) == 0 || len(source) > 10 {
		jsonError(w, "No source given", 400)
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
			Id:       r.Id,
			Hash:     r.Hash,
			FilePath: r.FilePath,
		}
	}

	resp := map[string]interface{}{"torrents": result}
	res := AjaxResult{"error", resp}
	raw, _ := json.Marshal(res)
	w.Write(raw)
}