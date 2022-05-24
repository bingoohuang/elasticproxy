package util

import (
	"golang.org/x/exp/constraints"
	"log"
	"net/url"
	"path"
)

func JoinURL(base *url.URL, target string) string {
	u, err := url.Parse(target)
	if err != nil {
		log.Printf("E! parse url %s failed: %s", target, err)
	}

	b := *base
	b.Path = path.Join(b.Path, u.Path)
	b.RawQuery = u.RawQuery

	return b.String()
}

func InRange[T constraints.Ordered](t, fromIncluded, toExcluded T) bool {
	return t >= fromIncluded && t < toExcluded
}
