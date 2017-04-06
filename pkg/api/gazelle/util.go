package gazelle

import (
	"errors"
	"fmt"
	"net/url"
)

var (
	errLoginFailed         = errors.New("Login failed")
	errRequestFailed       = errors.New("Request failed")
	errRequestFailedLogin  = errors.New("Request failed: not logged in")
	errRequestFailedReason = func(err string) error { return fmt.Errorf("Request failed: %s", err) }
	debugMode              = false
)

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
	if debugMode {
		fmt.Println(u.String())
	}
	return u.String(), nil
}

func checkResponseStatus(status, errorStr string) error {
	if status != "success" {
		if errorStr != "" {
			return errRequestFailedReason(errorStr)
		}
		return errRequestFailed
	}
	return nil
}
