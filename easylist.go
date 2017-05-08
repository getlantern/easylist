// Package easylist provides a library for loading easylist blocking rules and
// evaluating requests against them.
package easylist

import (
	"fmt"
	"io"
	"net"
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
			if len(rule.Parts) == 0 || rule.Parts[0].Type != adblock.DomainAnchor {
				// Only matching stuff anchored to domains
				continue
			}
			var matcher *adblock.RuleMatcher
			_u := rule.Parts[1].Value
			if strings.HasSuffix(_u, "%") {
				_u = _u[:len(_u)-1]
			}
			u, parseErr := url.Parse(fmt.Sprintf("http://%v", _u))
			if parseErr != nil {
				log.Errorf("Unable to parse %v: %v", _u, parseErr)
				skippedRules++
				continue
			}
			reversedDomain := reverse(u.Host)
			_matcher, found := domainMatchers[reversedDomain]
			if found {
				matcher = _matcher.(*adblock.RuleMatcher)
			} else {
				matcher = adblock.NewMatcher()
				domainMatchers[reversedDomain] = matcher
			}
			// Some options require knowledge that's not available outside of the
			// the browser. The adblock library currently ignores such options, but
			// we actually want to exclude these rules completely since we don't have
			// the information needed to properly process them.
			if rule.Opts.ObjectSubRequest != nil ||
				rule.Opts.Other != nil ||
				rule.Opts.SubDocument != nil ||
				rule.Opts.XmlHttpRequest != nil {
				log.Debugf("Skipping rule with unsupported option: %v", rule.Raw)
				skippedRules++
			} else {
				err = matcher.AddRule(rule, 0)
				if err != nil {
					skippedRules++
				} else {
					addedRules++
				}
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
	domain := withoutPort(req.Host)
	dm := l.getMatcher(reverse(domain))
	if dm == nil {
		// Until we've been initialized, allow everything
		return true
	}
	ar := &adblock.Request{
		URL:          req.URL.String(),
		Domain:       domain,
		OriginDomain: withoutPort(domainFromURL(req.Header.Get("Origin"))),
	}
	matched, _, _ := dm.Match(ar)
	return !matched
}

func (l *list) getMatcher(reversedDomain string) (dm *adblock.RuleMatcher) {
	l.mx.RLock()
	if l.domainMatchers == nil {
		l.mx.RUnlock()
		return
	}
	_, _dm, found := l.domainMatchers.LongestPrefix(reversedDomain)
	l.mx.RUnlock()
	if found {
		dm = _dm.(*adblock.RuleMatcher)
	}
	return
}

func domainFromURL(u string) string {
	if u == "" {
		return ""
	}
	_u, e := url.Parse(u)
	if e != nil {
		return ""
	}
	return _u.Host
}

func withoutPort(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return host
}

func reverse(input string) string {
	n := 0
	runes := make([]rune, len(input)+1)
	// Add a dot prefix to make sure we're only operating on subdomains
	runes[0] = '.'
	runes = runes[1:]
	for _, r := range input {
		runes[n] = r
		n++
	}
	runes = runes[0:n]
	// Reverse
	for i := 0; i < n/2; i++ {
		runes[i], runes[n-1-i] = runes[n-1-i], runes[i]
	}
	// Convert back to UTF-8.
	return string(runes)
}
