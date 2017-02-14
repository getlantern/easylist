// Package easylist provides a library for loading easylist blocking rules and
// evaluating requests against them.
package easylist

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/armon/go-radix"
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
		domainMatchers := make(map[string]interface{}, 1000)
		rules, err := adblock.ParseRules(r)
		if err != nil {
			return fmt.Errorf("Unable to parse rules: %v", err)
		}
		addedRules := 0
		skippedRules := 0
		for _, rule := range rules {
			if rule.Parts == nil || rule.Parts[0].Type != adblock.DomainAnchor {
				// Only matching stuff anchored to domains
				continue
			}
			var matcher *adblock.RuleMatcher
			_u := rule.Parts[1].Value
			if strings.HasSuffix(_u, "%") {
				_u = _u[:len(_u)-1]
			}
			u, parseErr := url.Parse(_u)
			if parseErr != nil {
				log.Errorf("Unable to parse %v: %v", _u, parseErr)
				skippedRules++
				continue
			}
			u.RawPath = ""
			u.RawQuery = ""
			domain := u.String()
			_matcher, found := domainMatchers[domain]
			if found {
				matcher = _matcher.(*adblock.RuleMatcher)
			} else {
				matcher = adblock.NewMatcher()
				domainMatchers[domain] = matcher
			}
			err = matcher.AddRule(rule, 0)
			if err != nil {
				skippedRules++
			} else {
				addedRules++
			}
		}

		dms := radix.NewFromMap(domainMatchers)
		l.mx.Lock()
		l.domainMatchers = dms
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
	domainMatchers *radix.Tree
	mx             sync.RWMutex
}

func (l *list) Allow(req *http.Request) bool {
	dm := l.getMatcher(req.Host)
	if dm == nil {
		// Until we've been initialized, allow everything
		return true
	}
	ar := &adblock.Request{
		URL:    req.URL.String(),
		Domain: req.Host,
	}
	matched, _, _ := dm.Match(ar)
	return !matched
}

func (l *list) getMatcher(domain string) (dm *adblock.RuleMatcher) {
	l.mx.RLock()
	_dm, found := l.domainMatchers.Get(domain)
	l.mx.RUnlock()
	if found {
		dm = _dm.(*adblock.RuleMatcher)
	}
	return
}
