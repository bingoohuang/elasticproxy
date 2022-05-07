package util

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoinURL(t *testing.T) {
	u, _ := url.Parse(`http://127.0.0.1:5003/backup`)
	j := JoinURL(u, "/person/_bulk?q=abc")
	assert.Equal(t, `http://127.0.0.1:5003/backup/person/_bulk?q=abc`, j)
}
