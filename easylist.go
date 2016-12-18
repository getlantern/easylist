// Package easylist provides a library for loading easylist blocking rules and
// evaluating requests against them.
package easylist

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/getlantern/golog"
	"github.com/getlantern/urlcache"
	"github.com/pmezard/adblock/adblock"
)

const (
	easylistURL = "https://easylist.to/easylist/easylist.txt"
)

var (
	log = golog.LoggerFor("easylist")
)

// List is a blocking list against which we can check HTTP requests to see
// whether or not they're allowed.
type List interface {
	// Allow checks whether this request is allowed based on the rules in this
	// list.
	Allow(req *http.Request) bool
}

// Open opens a new list, caching the data at cacheFile and checking for updates
// every checkInterval.
func Open(cacheFile string, checkInterval time.Duration) (List, error) {
	l := &list{}
	err := urlcache.Open(easylistURL, cacheFile, checkInterval, func(r io.Reader) error {
		matcher := adblock.NewMatcher()
		rules, err := adblock.ParseRules(r)
		if err != nil {
			return fmt.Errorf("Unable to parse rules: %v", err)
		}
		addedRules := 0
		skippedRules := 0
		for _, rule := range rules {
			err = matcher.AddRule(rule, 0)
			if err != nil {
				skippedRules++
			} else {
				addedRules++
			}
		}
		l.mx.Lock()
		l.matcher = matcher
		l.mx.Unlock()
		log.Debugf("Loaded new ruleset, added: %d   skipped: %d", addedRules, skippedRules)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return l, nil
}

type list struct {
	matcher *adblock.RuleMatcher
	mx      sync.RWMutex
}

func (l *list) Allow(req *http.Request) bool {
	m := l.getMatcher()
	if m == nil {
		// Until we've been initialized, allow everything
		return true
	}
	ar := &adblock.Request{
		URL:    req.URL.String(),
		Domain: req.Host,
	}
	matched, _, _ := m.Match(ar)
	return !matched
}

func (l *list) getMatcher() *adblock.RuleMatcher {
	l.mx.RLock()
	m := l.matcher
	l.mx.RUnlock()
	return m
}
