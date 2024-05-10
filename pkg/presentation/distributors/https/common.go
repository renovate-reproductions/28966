package https

import (
	"net/http"
	"strings"
)

type requestInfo struct {
	LanguagePreference []string
	Path               string
}

func extractRequestInfo(r *http.Request) (*requestInfo, error) {
	var ri requestInfo
	ri.LanguagePreference = getLanguagePreferenceFromHTTPHeaderAcceptLanguage(r.Header["Accept-Language"])
	ri.Path = r.URL.Path
	return &ri, nil
}

type requestInfoForBridge struct {
	BridgeType    string
	IPv6Requested bool
}

func extractRequestInfoForBridge(r *http.Request) (*requestInfoForBridge, error) {
	var ri requestInfoForBridge
	ri.BridgeType = r.URL.Query().Get("transport")
	ri.IPv6Requested = r.URL.Query().Get("ipv6") == "yes"
	return &ri, nil
}

func getLanguagePreferenceFromHTTPHeaderAcceptLanguage(headerValue []string) []string {
	if len(headerValue) == 0 {
		return []string{}
	}
	var languagePreference []string
	for _, v := range headerValue {
		requestedLanguages := strings.Split(v, ",")
		for _, requestedLanguage := range requestedLanguages {
			languageRequested := strings.Split(requestedLanguage, ";")[0]
			languagePreference = append(languagePreference, languageRequested)
		}
	}
	return languagePreference
}
