package util

import (
	"log"
	"net/http"
	"net/url"
)

func JoinURL(base *url.URL, target string) string {
	u, err := url.Parse(target)
	if err != nil {
		log.Printf("parse url %s failed: %s", target, err)
	}

	return base.ResolveReference(u).String()
}

var Client = &http.Client{}
