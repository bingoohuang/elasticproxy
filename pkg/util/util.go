package util

import (
	"log"
	"net/http"
	"net/url"
	"path"

	"golang.org/x/exp/constraints"
)

func JoinURL(base *url.URL, target string) string {
	u, err := url.Parse(target)
	if err != nil {
		log.Printf("parse url %s failed: %s", target, err)
	}

	b := *base
	b.Path = path.Join(b.Path, u.Path)
	b.RawQuery = u.RawQuery

	return b.String()
}

var Client = &http.Client{}

func InRange[T constraints.Ordered](t, fromIncluded, toExcluded T) bool {
	return t >= fromIncluded && t < toExcluded
}
