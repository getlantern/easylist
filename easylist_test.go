package easylist

import (
	"net/http"
	"os"
	"runtime/pprof"
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
	assert.True(t, l.Allow(req), "Domain that doesn't appear on list should be allowed")

	// Not currently checking non-domain-specific rules
	// req, _ = http.NewRequest("GET", "http://somedomain.com/adwords/stuff", nil)
	// assert.False(t, l.Allow(req))

	req, _ = http.NewRequest("GET", "https://cdn.adblade.com", nil)
	assert.False(t, l.Allow(req), "Domain on list should not be allowed")

	req, _ = http.NewRequest("GET", "https://c-sharpcorner.com/something/allowed", nil)
	assert.True(t, l.Allow(req), "Domain with path not matching rule should be allowed")

	req, _ = http.NewRequest("GET", "https://c-sharpcorner.com/stuff/banners/", nil)
	assert.False(t, l.Allow(req), "Domain with path matching rule should not be allowed")

	req, _ = http.NewRequest("GET", "https://s3.amazonaws.com", nil)
	assert.True(t, l.Allow(req), "Domain should be allowed by default")
	req.Header.Set("Origin", "https://gaybeeg.info")
	assert.False(t, l.Allow(req), "Domain loaded from specific domains should not be allowed")
}

func BenchmarkPass(b *testing.B) {
	l, err := Open("easylist.txt", 5*time.Minute)
	if err != nil {
		b.Fatalf("Unable to open easylist: %v", err)
	}

	// After loading rules, start CPU profiling
	os.Remove("benchmark.cpuprofile")
	cp, err := os.Create("benchmark.cpuprofile")
	if err != nil {
		b.Fatalf("Unable to create benchmark.cpuprofile: %v", err)
	}
	err = pprof.StartCPUProfile(cp)
	if err != nil {
		b.Fatalf("Unable to start CPU profiling: %v", err)
	}
	defer pprof.StopCPUProfile()

	b.ResetTimer()
	req, _ := http.NewRequest("GET", "http://osnews.com", nil)
	for i := 0; i < b.N; i++ {
		l.Allow(req)
	}
}
