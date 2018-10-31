package hybriddomain

import (
	"errors"
	"regexp"
	"strings"
)

const (
	// Keywords are reserved hybrid name

	KeywordHybrid = "hybrid"
	KeywordOver   = "over"
	KeywordWith   = "with"

	HybridSuffix = ".hybrid"

	nameRegexString = `^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`
)

var (
	ErrBadHybridDomain = errors.New("bad hybrid domain")

	nameRegex = regexp.MustCompile(nameRegexString)
)

func IsHybridName(name string) bool {
	switch name {
	case KeywordHybrid, KeywordOver, KeywordWith:
		return false
	default:
		return nameRegex.MatchString(name)
	}
}

// Domain changes when past to nodes:
//
// client got:
// GET http://192.168.22.22.over.a.b.c.hybrid/a
//
// a got:
// GET http://192.168.22.22.over.-a.b.c.hybrid/a
//
// b got:
// GET http://192.168.22.22.over.a.-b.c.hybrid/a
//
// c got:
// GET http://192.168.22.22.over.a.b.-c.hybrid/a
//
// c dial 192.168.22.22, then send:
// GET http://192.168.22.22/a
//
// if use `with` instead of `over`, c send:
// GET http://192.168.22.22.with.a.b.c.hybrid/a
//
// more:
// http://ipfs.with.hybrid/welcome
type Domain struct {
	// IsHybrid, if false, DialHostname and IsEnd are set.
	IsHybrid bool

	// IsOver request proxy type. Only support `over` and `with`.
	// If `over`, use DialHostname as Host in the end. If `with`, use
	// the begin host as Host in the end.
	IsOver bool

	// DialHostname always non-empty host to dial.
	DialHostname string

	// Next server to redirect to.
	Next string

	// IsBegin, the begin of proxy.
	IsBegin bool

	// IsEnd, the end of proxy. If false Next is non-empty.
	IsEnd bool

	// NextHostname should be set to request url and used by Next proxy.
	// Always non-empty.
	NextHostname string
}

func NewDomain(hostname string) (*Domain, error) {
	if !strings.HasSuffix(hostname, HybridSuffix) {
		return &Domain{
			IsHybrid:     false,
			DialHostname: hostname,
			IsEnd:        true,
		}, nil
	}

	overTag := "." + KeywordOver + "."
	withTag := "." + KeywordWith + "."
	overOrWith := &withTag
	overIdx := strings.LastIndex(hostname, withTag)
	if overIdx == -1 {
		overIdx = strings.LastIndex(hostname, overTag)
		if overIdx == -1 {
			return nil, ErrBadHybridDomain
		}
		overOrWith = &overTag
	}

	d := Domain{
		IsHybrid:     true,
		IsOver:       overOrWith == &overTag,
		DialHostname: hostname[:overIdx],
		IsBegin:      true,
	}

	hostnameLen := len(hostname)
	overOrWithLen := len(*overOrWith)
	hybridSuffixLen := len(HybridSuffix)

	routeStart := overIdx + overOrWithLen
	routeEnd := hostnameLen - hybridSuffixLen
	if routeStart > routeEnd {
		// 192.168.22.22.over.hybrid
		// no Next, but need NextHostname
		d.IsEnd = true
		if d.IsOver {
			// 192.168.22.22.over.hybrid => 192.168.22.22
			d.NextHostname = hostname[:hostnameLen-overOrWithLen-hybridSuffixLen+1]
		} else {
			// 192.168.22.22.with.hybrid
			d.NextHostname = hostname
		}
		return &d, nil
	}

	// a
	// -a
	// a.b.c
	// -a.b.c
	// a.-b.c
	// a.b.-c
	rawRouters := strings.Split(hostname[routeStart:routeEnd], ".")
	for i, rawRouter := range rawRouters {
		if rawRouter == "" {
			return nil, ErrBadHybridDomain
		}
		if rawRouter[0] == '-' {
			// not the Begin
			d.IsBegin = false
			rawRouters[i] = rawRouter[1:]
			if !IsHybridName(rawRouters[i]) {
				return nil, ErrBadHybridDomain
			}

			if i != len(rawRouters)-1 {
				d.Next = rawRouters[i+1]
				if !IsHybridName(d.Next) {
					return nil, ErrBadHybridDomain
				}
				rawRouters[i+1] = "-" + d.Next
			} else {
				d.IsEnd = true
			}
			break
		}
		if !IsHybridName(rawRouter) {
			return nil, ErrBadHybridDomain
		}
	}
	if d.IsBegin {
		d.Next = rawRouters[0]
		rawRouters[0] = "-" + d.Next
	}

	if d.IsOver && d.IsEnd {
		d.NextHostname = d.DialHostname
	} else {
		segments := []string{
			d.DialHostname,
			*overOrWith,
			strings.Join(rawRouters, "."),
			HybridSuffix,
		}
		d.NextHostname = strings.Join(segments, "")
	}
	return &d, nil
}
