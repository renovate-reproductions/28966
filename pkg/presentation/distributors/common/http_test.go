package common

import (
	"net/http"
	"testing"
)

func TestIpFromRequest(t *testing.T) {
	req := http.Request{
		RemoteAddr: "1.1.1.1:1",
		Header: map[string][]string{
			"X-Forwarded-For": {"127.0.0.1, 10.0.0.1, 2.2.2.2"},
		},
	}
	ip := IpFromRequest(&req, false)
	if ip.String() != "1.1.1.1" {
		t.Errorf("Wrong ip for no proxy trust: %s", ip.String())
	}

	ip = IpFromRequest(&req, true)
	if ip.String() != "2.2.2.2" {
		t.Errorf("Wrong ip with proxy trust: %s", ip.String())
	}
}
