package util

import (
	"net/http"
	"net/url"
	"path"
)

func JoinURL(base *url.URL, requestURI string) string {
	targetURL := *base
	targetURL.Path = path.Join(targetURL.Path, requestURI)
	return targetURL.String()
}

var Client = &http.Client{}
