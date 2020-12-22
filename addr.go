package nq

import (
	"fmt"
	"net"
	"net/url"
)

type addr struct {
	net, host string
}

func (a addr) Network() string { return a.net }
func (a addr) String() string  { return a.host }

// ParseURL parses net.Addr from net://addr URL (ex. "tcp4://localhost:8080" or "unix://:3999" )
func ParseURL(s string) (net.Addr, error) {
	u, err := url.Parse(s)
	errorf := func(format string, a ...interface{}) error {
		return fmt.Errorf("nanoq: parse %q: "+format, s, a)
	}
	if err != nil {
		return nil, errorf("%w", err)
	}
	if u.Opaque != "" {
		return nil, errorf("unexpected URL opaque %q", u.Opaque)
	}
	if u.User != nil {
		return nil, errorf("unexpected URL user %q", u.User)
	}
	if u.Path != "" && u.Path != "/" {
		return nil, errorf("unexpected URL path %q", u.Path)
	}
	if u.RawPath != "" && u.RawPath != "/" {
		return nil, errorf("unexpected URL raw path %q", u.RawPath)
	}
	if u.RawQuery != "" {
		return nil, errorf("unexpected URL rawquery %q", u.RawQuery)
	}
	if u.Fragment != "" {
		return nil, errorf("unexpected URL fragment %q", u.Fragment)
	}
	return addr{net: u.Scheme, host: u.Host}, nil
}

// MustParseURL is a panic flavored ParseURL
func MustParseURL(s string) net.Addr {
	addr, err := ParseURL(s)
	if err != nil {
		panic(err)
	}
	return addr
}
