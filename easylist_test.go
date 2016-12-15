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
	assert.True(t, l.Allow(req))

	req, _ = http.NewRequest("GET", "http://somedomain.com/adwords/stuff", nil)
	assert.False(t, l.Allow(req))

	req, _ = http.NewRequest("GET", "https://googleads.g.doubleclick.net", nil)
	assert.False(t, l.Allow(req))
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
