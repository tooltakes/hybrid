package hybriddomain

import (
	"testing"
)

type DomainSuccessTest struct {
	hostname string
	domain   Domain
}

var DomainSuccessTests = []DomainSuccessTest{
	{
		hostname: "192.168.22.22.over.a.b.c.hybrid",
		domain: Domain{
			IsHybrid:     true,
			IsOver:       true,
			DialHostname: "192.168.22.22",
			Next:         "a",   // a      b      c      ''
			IsBegin:      true,  // true   false  false  false
			IsEnd:        false, // false  false  false  true
			NextHostname: "192.168.22.22.over.-a.b.c.hybrid",
		},
	},
	{
		hostname: "192.168.22.22.with.-a.b.c.hybrid",
		domain: Domain{
			IsHybrid:     true,
			IsOver:       false,
			DialHostname: "192.168.22.22",
			Next:         "b",   // a      b      c      ''
			IsBegin:      false, // true   false  false  false
			IsEnd:        false, // false  false  false  true
			NextHostname: "192.168.22.22.with.a.-b.c.hybrid",
		},
	},
	{
		hostname: "192.168.22.1.over.a.-b.c.hybrid",
		domain: Domain{
			IsHybrid:     true,
			IsOver:       true,
			DialHostname: "192.168.22.1",
			Next:         "c",   // a      b      c      ''
			IsBegin:      false, // true   false  false  false
			IsEnd:        false, // false  false  false  true
			NextHostname: "192.168.22.1.over.a.b.-c.hybrid",
		},
	},
	{
		hostname: "192.168.22.22.over.a.b.-c.hybrid",
		domain: Domain{
			IsHybrid:     true,
			IsOver:       true,
			DialHostname: "192.168.22.22",
			Next:         "",    // a      b      c      ''
			IsBegin:      false, // true   false  false  false
			IsEnd:        true,  // false  false  false  true
			NextHostname: "192.168.22.22",
		},
	},
	{
		hostname: "192.168.22.22.with.a.b.-c.hybrid",
		domain: Domain{
			IsHybrid:     true,
			IsOver:       false,
			DialHostname: "192.168.22.22",
			Next:         "",    // a      b      c      ''
			IsBegin:      false, // true   false  false  false
			IsEnd:        true,  // false  false  false  true
			NextHostname: "192.168.22.22.with.a.b.c.hybrid",
		},
	},
	{
		hostname: "192.168.22.22.over.a.hybrid",
		domain: Domain{
			IsHybrid:     true,
			IsOver:       true,
			DialHostname: "192.168.22.22",
			Next:         "a",
			IsBegin:      true,
			IsEnd:        false,
			NextHostname: "192.168.22.22.over.-a.hybrid",
		},
	},
	{
		hostname: "192.168.22.22.over.-a.hybrid",
		domain: Domain{
			IsHybrid:     true,
			IsOver:       true,
			DialHostname: "192.168.22.22",
			Next:         "",
			IsBegin:      false,
			IsEnd:        true,
			NextHostname: "192.168.22.22",
		},
	},
	{
		hostname: "192.168.22.22.with.-a.hybrid",
		domain: Domain{
			IsHybrid:     true,
			IsOver:       false,
			DialHostname: "192.168.22.22",
			Next:         "",
			IsBegin:      false,
			IsEnd:        true,
			NextHostname: "192.168.22.22.with.a.hybrid",
		},
	},
	{
		hostname: "192.168.22.22.over.hybrid",
		domain: Domain{
			IsHybrid:     true,
			IsOver:       true,
			DialHostname: "192.168.22.22",
			Next:         "",
			IsBegin:      true,
			IsEnd:        true,
			NextHostname: "192.168.22.22",
		},
	},
	{
		hostname: "192.168.22.22.with.hybrid",
		domain: Domain{
			IsHybrid:     true,
			IsOver:       false,
			DialHostname: "192.168.22.22",
			Next:         "",
			IsBegin:      true,
			IsEnd:        true,
			NextHostname: "192.168.22.22.with.hybrid",
		},
	},

	{
		hostname: "192.168.22.22.with",
		domain: Domain{
			IsHybrid:     false,
			DialHostname: "192.168.22.22.with",
		},
	},
	{
		hostname: "192.168.22.22",
		domain: Domain{
			IsHybrid:     false,
			DialHostname: "192.168.22.22",
		},
	},
	{
		hostname: "192.168.22.22.with.a-hybrid",
		domain: Domain{
			IsHybrid:     false,
			DialHostname: "192.168.22.22.with.a-hybrid",
		},
	},
}

func TestNewDomainSuccess(t *testing.T) {
	for _, st := range DomainSuccessTests {
		domain, err := NewDomain(st.hostname)
		if err != nil {
			t.Errorf("NewDomain(%q) got err: %v", st.hostname, err)
		} else if *domain != st.domain {
			t.Errorf("NewDomain(%q) = %v, want %v", st.hostname, *domain, st.domain)
		}
	}
}

var DomainFailTests = []string{
	"192.168.22.22.hybrid",
	"192.168.22.22.with-a.hybrid",
	"192.168.22.22.with..hybrid",
	"192.168.22.22.with.a..hybrid",
	"192.168.22.22.with.-.hybrid",
	"192.168.22.22.with.a_0.hybrid",
}

func TestNewDomainFail(t *testing.T) {
	for _, ft := range DomainFailTests {
		_, err := NewDomain(ft)
		if err == nil {
			t.Errorf("NewDomain(%q) should get err", ft)
		}
	}
}
