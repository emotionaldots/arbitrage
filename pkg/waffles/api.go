package waffles

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/kdvh/whatapi"
)

var (
	errLoginFailed         = errors.New("Login failed")
	errRequestFailed       = errors.New("Request failed")
	errRequestFailedLogin  = errors.New("Request failed: not logged in")
	errRequestFailedReason = func(err string) error { return fmt.Errorf("Request failed: %s", err) }
)

func NewAPI(url, agent string) (*API, error) {
	w := &API{}
	w.baseURL = url
	w.userAgent = agent
	cookieJar, err := cookiejar.New(nil)
	if err != nil {
		return w, err
	}
	w.client = &http.Client{Jar: cookieJar}
	return w, err
}

type API struct {
	baseURL   string
	userAgent string
	client    *http.Client
	passkey   string
	uid       string
	loggedIn  bool
}

func (w *API) Login(username, password string) error {
	params := url.Values{}
	params.Set("_username", username)
	params.Set("_password", password)

	reqBody := strings.NewReader(params.Encode())
	req, err := http.NewRequest("POST", w.baseURL+"login_check", reqBody)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", w.userAgent)
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errLoginFailed
	}
	if resp.Request.URL.String()[len(w.baseURL):] == "login_check" {
		return errLoginFailed
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return err
	}
	uidString, ok := doc.Find("span.hname a").Attr("href")
	if !ok {
		return errors.New("Parsing failed: could not find uid field")
	}
	uidURL, err := url.Parse(uidString)
	if err != nil {
		return err
	}
	uid := uidURL.Query().Get("id")
	if uid == "" {
		return errors.New("Parsing failed: empty userid")
	}

	w.loggedIn = true
	return nil
}

func (w *API) doRequest(requestURL string) ([]byte, error) {
	if !w.loggedIn {
		return nil, errRequestFailedLogin
	}

	req, err := http.NewRequest("GET", requestURL, nil)
	req.Header.Set("User-Agent", w.userAgent)
	if err != nil {
		return nil, err
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errRequestFailedReason("Status Code " + resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return body, err
	}

	return body, nil
}

func (w *API) DoTorrent(id int, params url.Values) ([]byte, error) {
	params.Set("id", strconv.Itoa(id))
	params.Set("filelist", "1")
	requestURL, err := buildURL(w.baseURL, "details.php", "", params)
	if err != nil {
		return nil, err
	}
	return w.doRequest(requestURL)
}

func buildURL(baseURL, path, action string, params url.Values) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	u.Path = path
	query := make(url.Values)
	if action != "" {
		query.Set("action", action)
	}
	for param, values := range params {
		for _, value := range values {
			query.Set(param, value)
		}
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}

type artist struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (w *API) ParseTorrent(body []byte) (whatapi.Torrent, error) {
	r := whatapi.Torrent{}

	b := bytes.NewReader(body)
	doc, err := goquery.NewDocumentFromReader(b)
	if err != nil {
		return r, err
	}

	// Filelist
	row := doc.Find("a[name=filelist]").Parent().Parent()
	if row.Length() == 0 {
		return r, errors.New("Parsing failed: no filelist found")
	}

	files := ""
	var size int64
	row.Find("table tr").Each(func(i int, s *goquery.Selection) {
		if i == 0 {
			return
		}
		name := s.Children().First().Text()
		size, err = parseBytes(s.Children().Last().Text())
		files += name + "{{{" + strconv.FormatInt(size, 10) + "}}}|||"
	})
	files = strings.TrimRight(files, "|")
	if err != nil {
		return r, err
	}
	r.Torrent.FileList = files

	mainTable := row.Closest("tbody")
	tbl := tabular(mainTable)

	// Artist
	artistField, ok := tbl["Artist"]
	if !ok {
		return r, errors.New("Parsing failed: no artist found")
	}
	artistText := artistField.Find("a").Text()
	r.Group.MusicInfo.Artists = append(r.Group.MusicInfo.Artists, artist{
		Name: artistText,
	})

	// Description
	desc, err := tbl["Description"].Html()
	if err != nil {
		return r, err
	}
	r.Group.WikiBody = desc

	// Torrent Info
	title := mainTable.Parent().PrevAllFiltered("h1").Text()
	if title == "" {
		return r, errors.New("Parsing failed: torrent title not found")
	}
	if err := ParseTitle(artistText, title, &r); err != nil {
		return r, err
	}

	cat := tbl["Type"].Text()
	r.Group.Tags = append(r.Group.Tags, cat)

	return r, nil
}

var reTitle = regexp.MustCompile(`(.+) \[([^\]]+)\]$`)

func ParseTitle(artist, title string, r *whatapi.Torrent) error {
	if title == artist {
		r.Group.Name = title
		return nil
	}
	if strings.HasPrefix(title, artist+" - ") {
		title = strings.TrimPrefix(title, artist+" - ")
	}

	mainParts := strings.SplitN(title, " - ", 2)
	if len(mainParts) == 2 {
		title = mainParts[2]
	}
	matches := reTitle.FindStringSubmatch(title)
	if matches == nil {
		r.Group.Name = title
		return nil
	}

	r.Group.Name = matches[1]
	tags := strings.Split(matches[2], "/")
	for _, tag := range tags {
		switch {
		case tag == "MP3" || tag == "FLAC" || tag == "DTS" || tag == "AC3":
			r.Torrent.Format = tag
		case tag == "CD" || tag == "Vinyl" || tag == "Cassette" || tag == "Web":
			r.Torrent.Media = tag
		case tag == "Log":
			r.Torrent.HasLog = true
		case isEncoding(tag):
			r.Torrent.Encoding = tag
		case len(tag) == 4:
			year, err := strconv.Atoi(tag)
			if err == nil {
				r.Group.Year = year
			}
		}
	}

	return nil
}

func isEncoding(v string) bool {
	if v == "Lossless" {
		return true
	}
	if v == "64" || v == "128" || v == "192" || v == "256" || v == "320" {
		return true
	}
	if strings.HasSuffix(v, "(VBR)") {
		return true
	}
	return false
}

func tabular(table *goquery.Selection) map[string]*goquery.Selection {
	rows := make(map[string]*goquery.Selection)
	table.Children().Each(func(i int, s *goquery.Selection) {
		name := s.Children().First().Text()
		content := s.Children().Last()
		rows[name] = content
	})
	return rows
}

func parseBytes(s string) (int64, error) {
	parts := strings.SplitN(s, " ", 2)
	if len(parts) == 1 {
		return -1, fmt.Errorf("Could not parse byte string %q: invalid parts", s)
	}

	n, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return -1, fmt.Errorf("Could not parse byte string %q: %s", s, err)
	}
	i := int64(n * 1024)
	switch parts[1] {
	case "GB":
		i *= 1024 * 1024
	case "MB":
		i *= 1024
	case "kB":
		i *= 1
	default:
		return -1, fmt.Errorf("Could not parse byte string %q: unknown unit", s)
	}

	return i, nil
}
