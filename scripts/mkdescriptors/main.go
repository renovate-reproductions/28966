// Copyright (c) 2023, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"embed"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"path"
	"strings"
	"text/template"
	"time"
)

const (
	NUM_BRIDGES = 1000
)

//go:embed *.tmpl
var tmplFS embed.FS
var random *rand.Rand

type Bridge struct {
	Nick           string
	Address        string
	OrPort         int
	PTPort         int
	Fingerprint    string
	B64Fingerprint string
	Transport      string
	Args           string
	Flags          string
}

func main() {
	random = rand.New(rand.NewSource(time.Now().UnixNano()))

	numBridges := flag.Int("n", NUM_BRIDGES, "Number of bridges to generate")
	flag.Parse()
	folder := flag.Arg(0)
	if folder == "" {
		folder = "."
	}
	err := os.MkdirAll(folder, 0750)
	if err != nil {
		log.Fatal(err)
	}

	bridges := make([]Bridge, 0, *numBridges)
	for i := 0; i < NUM_BRIDGES; i++ {
		nick := fmt.Sprintf("Bridge%05d", i)
		bridges = append(bridges, genBridge(nick))
	}

	funcMap := template.FuncMap{
		"splitFingerprint": splitFingerprint,
	}
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(tmplFS, "*")
	if err != nil {
		log.Fatal(err)
	}

	for _, descriptor := range []string{"bridge-descriptors", "cached-extrainfo", "networkstatus-bridges"} {
		f, err := os.Create(path.Join(folder, descriptor))
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		tmpl.ExecuteTemplate(f, descriptor+".tmpl", bridges)
	}
	os.Link(path.Join(folder, "cached-extrainfo"), path.Join(folder, "cached-extrainfo.new"))
}

func genBridge(nick string) Bridge {
	hexfp, b64fp := genFingerprint()
	return Bridge{
		Nick:           nick,
		Address:        genIP().String(),
		OrPort:         random.Intn(65536),
		PTPort:         random.Intn(65536),
		Fingerprint:    hexfp,
		B64Fingerprint: b64fp,
		Transport:      "obfs4",
		Args:           "cert=ZZZZZZZZZZZ,iat-mode=0",
		Flags:          "V2Dir Valid",
	}
}

func genIP() (ip net.IP) {
	for ip == nil || ip.IsUnspecified() || ip.IsPrivate() || ip.IsLoopback() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalUnicast() {

		bytes := make([]byte, 4)
		_, err := random.Read(bytes)
		if err != nil {
			log.Fatal(err)
		}
		ip = net.IPv4(bytes[0], bytes[1], bytes[2], bytes[3])
	}
	return
}

func genFingerprint() (hexfp string, base64fp string) {
	bytes := make([]byte, 20)
	_, err := random.Read(bytes)
	if err != nil {
		log.Fatal(err)
	}
	hexfp = strings.ToUpper(hex.EncodeToString(bytes))

	encoded := base64.StdEncoding.EncodeToString(bytes)
	base64fp = strings.TrimRight(encoded, "=")
	return
}

func splitFingerprint(fp string) string {
	splitedFp := ""
	for i := 4; i < len(fp); i += 4 {
		splitedFp += fp[i-4:i] + " "
	}
	splitedFp += fp[len(fp)-4:]
	return splitedFp
}
