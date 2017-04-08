package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	client    *http.Client
	Url       string
	UserAgent string
	LastTime  time.Time
}

func New(url, agent string) *Client {
	return &Client{
		client:    &http.Client{},
		Url:       strings.TrimRight(url, "/"),
		UserAgent: agent,
		LastTime:  time.Now().Add(-2 * time.Second),
	}
}

type Release struct {
	Id       int64  `json:"id"`
	Hash     string `json:"hash"`
	FilePath string `json:"filePath"`
}

type queryRequest struct {
	Source string   `json:"source"`
	Hashes []string `json:"hashes"`
}

type Response struct {
	Status string           `json:"status"`
	Error  string           `json:"error"`
	Result *json.RawMessage `json:"response"`
}

func (r Response) IsErr() error {
	if r.Status == "success" {
		return nil
	}
	return errors.New("API returned error: " + r.Error)
}

func (c *Client) Query(source string, hashes []string) ([]Release, error) {
	time.Sleep(c.LastTime.Add(2 * time.Second).Sub(time.Now()))
	c.LastTime = time.Now()

	if source == "" {
		return nil, errors.New("api query: empty source")
	}
	if len(hashes) == 0 {
		return nil, errors.New("api query: empty hashes")
	}

	params := url.Values{}
	params.Set("source", source)

	endpoint := c.Url + "/api/query_batch"
	if len(hashes) == 1 {
		params.Set("hash", hashes[0])
		endpoint = c.Url + "/api/query"
	} else {
		params["hashes"] = hashes
	}

	reqBody := strings.NewReader(params.Encode())
	req, err := http.NewRequest("POST", endpoint, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		if resp.StatusCode != 200 {
			err = fmt.Errorf("api query: unexpected status code %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		}
		return nil, err
	}
	if err := result.IsErr(); err != nil {
		return nil, err
	}

	var releases []Release
	err = json.Unmarshal(*result.Result, &releases)
	return releases, err
}
