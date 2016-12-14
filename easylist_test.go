package easylist

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBlock(t *testing.T) {
	l, err := Open("easylist.txt", 5*time.Minute)
	if !assert.NoError(t, err) {
		return
	}

	req, _ := http.NewRequest("GET", "http://osnews.com", nil)
	assert.True(t, l.Allow(req))

	req, _ = http.NewRequest("GET", "http://somedomain.com/adwords/stuff", nil)
	assert.False(t, l.Allow(req))

	req, _ = http.NewRequest("GET", "https://googleads.g.doubleclick.net", nil)
	assert.False(t, l.Allow(req))
}
