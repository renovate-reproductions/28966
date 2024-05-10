// Copyright (c) 2023, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"testing"
)

func TestExampleConfig(t *testing.T) {
	config, err := LoadConfig("../conf/config.json")
	if err != nil {
		t.Fatal("Can't load example config:", err)
	}

	if config.Backend.AssignmentsFile != "assignments.log" {
		t.Error("Wrong assignments file:", config.Backend.AssignmentsFile)
	}
}

func TestMultipleConfigFiles(t *testing.T) {
	var config Config
	err := config.Set("../conf/config.json")
	if err != nil {
		t.Fatal("Can't load example config:", err)
	}

	err = config.Set("./test_assets/secrets.json")
	if err != nil {
		t.Fatal("Can't load secrets config:", err)
	}

	if config.Backend.BridgestrapToken != "BridgestrapSecret" {
		t.Error("Wrong bridgestrap token:", config.Backend.BridgestrapToken)
	}
	if config.Backend.ApiTokens["https"] != "HttpsSecret" {
		t.Error("Wrong https token:", config.Backend.ApiTokens["https"])
	}
	if config.Backend.StorageDir != "storage" {
		t.Error("Wrong storage dir:", config.Backend.StorageDir)
	}
}
