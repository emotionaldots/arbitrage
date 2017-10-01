package gazelle

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"

	"github.com/emotionaldots/arbitrage/pkg/model"
	"github.com/emotionaldots/arbitrage/pkg/model/fixes"
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
	authkey   string
	passkey   string
	loggedIn  bool
}

func (w *API) GetJSON(requestURL string, responseObj interface{}) error {
	if !w.loggedIn {
		return errRequestFailedLogin
	}

	req, err := http.NewRequest("GET", requestURL, nil)
	req.Header.Set("User-Agent", w.userAgent)
	if err != nil {
		return err
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errRequestFailedReason("Status Code " + resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var st Response
	if err := json.Unmarshal(body, &st); err != nil {
		return err
	}

	if err := checkResponseStatus(st.Status, st.Error); err != nil {
		return err
	}
	return json.Unmarshal([]byte(*st.Result), responseObj)
}

type Response struct {
	Status string           `json:"status"`
	Error  string           `json:"error"`
	Result *json.RawMessage `json:"response"`
}

func (w *API) Do(action string, params url.Values, result interface{}) error {
	requestURL, err := buildURL(w.baseURL, "ajax.php", action, params)
	if err != nil {
		return err
	}
	return w.GetJSON(requestURL, result)
}

func (w *API) CreateDownloadURL(id int) (string, error) {
	if !w.loggedIn {
		return "", errRequestFailedLogin
	}

	params := url.Values{}
	params.Set("action", "download")
	params.Set("id", strconv.Itoa(id))
	params.Set("authkey", w.authkey)
	params.Set("torrent_pass", w.passkey)
	downloadURL, err := buildURL(w.baseURL, "torrents.php", "", params)
	if err != nil {
		return "", err
	}
	return downloadURL, nil
}

func (w *API) Login(username, password string) error {
	params := url.Values{}
	params.Set("username", username)
	params.Set("password", password)

	reqBody := strings.NewReader(params.Encode())
	req, err := http.NewRequest("POST", w.baseURL+"login.php", reqBody)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", w.userAgent)
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	if resp.Request.URL.String()[len(w.baseURL):] != "index.php" {
		return errLoginFailed
	}
	w.loggedIn = true
	account, err := w.GetAccount()
	if err != nil {
		return err
	}
	w.authkey, w.passkey = account.AuthKey, account.PassKey
	return nil
}

func (w *API) Logout() error {
	params := url.Values{"auth": {w.authkey}}
	requestURL, err := buildURL(w.baseURL, "logout.php", "", params)
	if err != nil {
		return err
	}
	_, err = w.client.Get(requestURL)
	if err != nil {
		return err
	}
	w.loggedIn, w.authkey, w.passkey = false, "", ""
	return nil
}

func (w *API) GetAccount() (model.Account, error) {
	var result model.Account
	err := w.Do("index", url.Values{}, &result)
	return result, err
}

func (w *API) GetTorrent(id int, params url.Values) (model.TorrentAndGroup, error) {
	var result model.TorrentAndGroup
	params.Set("id", strconv.Itoa(id))
	err := w.Do("torrent", params, &result)
	return result, err
}

func (w *API) GetTorrentGroup(id int, params url.Values) (model.GroupAndTorrents, error) {
	var result model.GroupAndTorrents
	params.Set("id", strconv.Itoa(id))
	err := w.Do("torrentgroup", params, &result)
	return result, err
}

func (w *API) GetCollage(id int, params url.Values) (model.CollageWithGroups, error) {
	var result fixes.CollageWithStringedGroups
	params.Set("id", strconv.Itoa(id))
	err := w.Do("collage", params, &result)
	return result.Fix(), err
}
