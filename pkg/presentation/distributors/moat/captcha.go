package moat

import _ "embed"

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/presentation/distributors/common"
)

//go:embed captcha.jpg
var captchaImage []byte

type captchaFetchRequest struct {
	Data []captchaFetchRequestData `json:"data"`
}

type captchaFetchRequestData struct {
	Supported []string `json:"supported"`
}

type captchaFetchResponse struct {
	Data []captchaFetchResponseData `json:"data"`
}

type captchaFetchResponseData struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	Version   string   `json:"version"`
	Transport []string `json:"transport"`
	Image     string   `json:"image"`
	Challenge string   `json:"challenge"`
}

func (mh moatHandler) captchaFetchHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)

	var request captchaFetchRequest
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&request)
	if err != nil && !errors.Is(err, io.EOF) {
		log.Println("Error decoding captcha fetch request:", err)
		err = enc.Encode(invalidRequest)
		if err != nil {
			log.Println("Error encoding jsonError:", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	transports := []string{}
	if len(request.Data) == 0 || len(request.Data[0].Supported) == 0 {
		transports = mh.cfg.Resources
	} else {
		for _, t := range request.Data[0].Supported {
			for _, r := range mh.cfg.Resources {
				if t == r {
					transports = append(transports, t)
					break
				}
			}
		}
	}
	if len(transports) == 0 {
		log.Println("No valid transports provided in captcha fetch:", request)
		err = enc.Encode(invalidRequest)
		return
	}

	response := captchaFetchResponse{
		Data: []captchaFetchResponseData{
			{
				ID:        "1",
				Type:      "moat-challenge",
				Version:   "0.1.0",
				Transport: transports,
				Image:     base64.StdEncoding.EncodeToString(captchaImage),
				Challenge: strings.Join(transports, "|"),
			},
		},
	}

	err = enc.Encode(response)
	if err != nil {
		log.Println("Error encoding circumvention defaults:", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

type captchaCheckRequest struct {
	Data []captchaCheckRequestData `json:"data"`
}

type captchaCheckRequestData struct {
	Challenge string `json:"challenge"`
}

type captchaCheckResponse struct {
	Data []captchaCheckResponseData `json:"data"`
}

type captchaCheckResponseData struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Version string   `json:"version"`
	Bridges []string `json:"bridges"`
	QRCode  *string  `json:"qrcode"`
}

func (mh moatHandler) captchaCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)

	var request captchaCheckRequest
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&request)
	if err != nil && !errors.Is(err, io.EOF) {
		log.Println("Error decoding captcha fetch request:", err)
		err = enc.Encode(invalidRequest)
		if err != nil {
			log.Println("Error encoding jsonError:", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	if len(request.Data) == 0 {
		log.Println("No data provided in captcha check:", request)
		err = enc.Encode(invalidRequest)
		return
	}

	transports := strings.Split(request.Data[0].Challenge, "|")
	if len(transports) == 0 {
		log.Println("No transports in captcha check:", request)
		err = enc.Encode(invalidRequest)
		return
	}
	ip := common.IpFromRequest(r, mh.cfg.TrustProxy)
	bridges := mh.dist.GetBridges(transports[0], ip)

	response := captchaCheckResponse{
		Data: []captchaCheckResponseData{
			{
				ID:      "1",
				Type:    "moat-challenge",
				Version: "0.1.0",
				Bridges: bridges,
			},
		},
	}

	err = enc.Encode(response)
	if err != nil {
		log.Println("Error encoding circumvention defaults:", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
