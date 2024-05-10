package common

import (
	"net"
	"net/http"
	"strings"
)

func IpFromRequest(r *http.Request, trustProxy bool) net.IP {
	var ip net.IP

	if trustProxy {
		header := r.Header.Get("X-Forwarded-For")
		forwarded := strings.Split(header, ",")
		for _, f := range forwarded {
			ip = net.ParseIP(strings.TrimSpace(f))
			if ip == nil {
				continue
			}
			if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsPrivate() {
				ip = nil
				continue
			}
			break
		}
	}

	if ip == nil {
		// if no X-Forwarded-For header let's take the IP from the request
		ipStr := strings.Split(r.RemoteAddr, ":")[0]
		ip = net.ParseIP(ipStr)
		if ip == nil {
			return ip
		}
	}

	return ip
}
