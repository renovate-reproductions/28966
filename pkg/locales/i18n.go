// Copyright (c) 2023, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package locales

import (
	"embed"
	"encoding/json"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

const (
	DefaultLanguage = "en"
)

//go:embed *.json
var localeFS embed.FS

func NewBundle() (*i18n.Bundle, error) {
	files, err := localeFS.ReadDir(".")
	if err != nil {
		return nil, err
	}

	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	for _, file := range files {
		bundle.LoadMessageFileFS(localeFS, file.Name())
	}
	return bundle, nil
}
