// Copyright (c) 2021-2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package https

import (
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"rsc.io/qr"

	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/presentation/distributors/common"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/distributors/https"
)

var dist *https.HttpsDistributor

type bridgeRequestHandler struct {
	cfg *internal.Config
}

func (b *bridgeRequestHandler) RequestHandler(w http.ResponseWriter, r *http.Request) {
	bridgeRequest, err := extractRequestInfoForBridge(r)
	if err != nil {
		http.RedirectHandler("static/error.html", http.StatusTemporaryRedirect).ServeHTTP(w, r)
		log.Printf("Error extracting request info for bridge: %s", err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	resources, err := dist.RequestBridges(bridgeRequest.BridgeType, common.IpFromRequest(r, b.cfg.Distributors.Https.TrustProxy), bridgeRequest.IPv6Requested)
	if err != nil {
		http.RedirectHandler("static/error.html", http.StatusTemporaryRedirect).ServeHTTP(w, r)
		log.Printf("Error requesting bridges: %s", err)
		return
	}
	data, err := json.Marshal(resources)
	if err != nil {
		http.RedirectHandler("static/error.html", http.StatusTemporaryRedirect).ServeHTTP(w, r)
		log.Printf("Error marshalling resources: %s", err)
		return
	}
	qrcode, err := qr.Encode(string(data), qr.M)
	if err != nil {
		http.RedirectHandler("static/error.html", http.StatusTemporaryRedirect).ServeHTTP(w, r)
		log.Printf("Error encoding QR code: %s", err)
		return
	}
	if resources == nil || len(resources) == 0 {
		resources = []string{"No bridges available"}
	}
	qrcodeInPNGInBase64 := base64.StdEncoding.EncodeToString(qrcode.PNG())
	renderPage(w, r, "bridges.html", map[string]interface{}{
		"BridgeLines": resources,
		"QRCode":      qrcodeInPNGInBase64,
	})
}

func renderPage(w http.ResponseWriter, r *http.Request, page string, input map[string]interface{}) {
	request, err := extractRequestInfo(r)
	if err != nil {
		http.RedirectHandler("static/error.html", http.StatusTemporaryRedirect).ServeHTTP(w, r)
		log.Printf("Error extracting request info: %s", err)
		return
	}
	context, err := newRenderingContextWithOpts(request.LanguagePreference)
	if err != nil {
		http.RedirectHandler("static/error.html", http.StatusTemporaryRedirect).ServeHTTP(w, r)
		log.Printf("Error creating rendering context: %s", err)
		return
	}
	err = context.render(page, input, w)
	if err != nil {
		http.RedirectHandler("static/error.html", http.StatusTemporaryRedirect).ServeHTTP(w, r)
		log.Printf("Error rendering template: %s", err)
		return
	}
}

func RequestHandleWith(path string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		path = strings.TrimPrefix(path, "/")
		renderPage(w, r, path, nil)
	}
}

// InitFrontend is the entry point to HTTPS's Web frontend.  It spins up the
// Web server and then waits until it receives a SIGINT.
func InitFrontend(cfg *internal.Config) {

	dist = &https.HttpsDistributor{}
	bridgeReq := bridgeRequestHandler{cfg: cfg}
	handlers := map[string]http.HandlerFunc{
		"/":        http.HandlerFunc(RequestHandleWith("homepage.html")),
		"/options": http.HandlerFunc(RequestHandleWith("options.html")),
		"/bridges": http.HandlerFunc(bridgeReq.RequestHandler),
		"/static/": func(writer http.ResponseWriter, request *http.Request) {
			subEmbeddedFS, _ := fs.Sub(embedfs, "embedded")
			http.FileServer(http.FS(subEmbeddedFS)).ServeHTTP(writer, request)
		},
		"/howto": http.RedirectHandler("https://tb-manual.torproject.org/bridges/", http.StatusTemporaryRedirect).ServeHTTP,
		"/info":  http.RedirectHandler("https://tb-manual.torproject.org/bridges/", http.StatusTemporaryRedirect).ServeHTTP,
	}

	common.StartWebServer(
		&cfg.Distributors.Https.WebApi,
		cfg,
		dist,
		handlers,
	)
}
