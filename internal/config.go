// Copyright (c) 2021-2024, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"encoding/json"
	"log"
	"net"
	"os"
)

// Config represents our central configuration file.
type Config struct {
	Backend      BackendConfig `json:"backend"`
	Distributors Distributors  `json:"distributors"`
	Updaters     Updaters      `json:"updaters"`
	isIntialized bool
}

type BackendConfig struct {
	ExtrainfoFile           string            `json:"extrainfo_file"`
	NetworkstatusFile       string            `json:"networkstatus_file"`
	DescriptorsFile         string            `json:"descriptors_file"`
	BlocklistFile           string            `json:"blocklist_file"`
	AllowlistFile           string            `json:"allowlist_file"`
	ApiTokens               map[string]string `json:"api_tokens"`
	ResourcesEndpoint       string            `json:"api_endpoint_resources"`
	ResourceStreamEndpoint  string            `json:"api_endpoint_resource_stream"`
	TargetsEndpoint         string            `json:"api_endpoint_targets"`
	StatusEndpoint          string            `json:"web_endpoint_status"`
	MetricsEndpoint         string            `json:"web_endpoint_metrics"`
	BridgestrapEndpoint     string            `json:"bridgestrap_endpoint"`
	BridgestrapToken        string            `json:"bridgestrap_token"`
	OnbascaEndpoint         string            `json:"onbasca_endpoint"`
	OnbascaToken            string            `json:"onbasca_token"`
	BandwidthRatioThreshold float64           `json:"bandwidth_ratio_threshold"`
	StorageDir              string            `json:"storage_dir"`
	AssignmentsFile         string            `json:"assignments_file"`
	// DistProportions contains the proportion of resources that each
	// distributor should get.  E.g. if the HTTPS distributor is set to x and
	// the moat distributor is set to y, then HTTPS gets x/(x+y) of all
	// resources and moat gets y/(x+y).
	DistProportions map[string]int            `json:"distribution_proportions"`
	Resources       map[string]ResourceConfig `json:"resources"`
	WebApi          WebApiConfig              `json:"web_api"`
}

type ResourceConfig struct {
	Unpartitioned bool     `json:"unpartitioned"`
	Stored        bool     `json:"stored"`
	Distributors  []string `json:"distributors"`
}

type Distributors struct {
	Https    HttpsDistConfig    `json:"https"`
	Email    EmailDistConfig    `json:"email"`
	Stub     StubDistConfig     `json:"stub"`
	Gettor   GettorDistConfig   `json:"gettor"`
	Moat     MoatDistConfig     `json:"moat"`
	Telegram TelegramDistConfig `json:"telegram"`
	Whatsapp WhatsAppConfig     `json:"whatsapp"`
}

type StubDistConfig struct {
	Resources []string     `json:"resources"`
	WebApi    WebApiConfig `json:"web_api"`
}

type HttpsDistConfig struct {
	Resources        []string               `json:"resources"`
	WebApi           WebApiConfig           `json:"web_api"`
	TimeDistribution TimeDistributionConfig `json:"time_distribution"`
	TrustProxy       bool                   `json:"trust_proxy"`
}

type EmailDistConfig struct {
	Resources            []string    `json:"resources"`
	NumBridgesPerRequest int         `json:"num_bridges_per_request"`
	RotationPeriodHours  int         `json:"rotation_period_hours"`
	AllowedDomains       []string    `json:"allowed_domains"`
	Email                EmailConfig `json:"email"`
	MetricsAddress       string      `json:"metrics_address"`
}

type GettorDistConfig struct {
	Resources      []string    `json:"resources"`
	Email          EmailConfig `json:"email"`
	MetricsAddress string      `json:"metrics_address"`
}

type MoatDistConfig struct {
	Resources             []string               `json:"resources"`
	GeoipDB               string                 `json:"geoipdb"`
	Geoip6DB              string                 `json:"geoip6db"`
	CircumventionMap      string                 `json:"circumvention_map"`
	CircumventionDefaults string                 `json:"circumvention_defaults"`
	BuiltInBridgesURL     string                 `json:"builtin_bridges_url"`
	ShimTokens            map[string]string      `json:"shim_tokens"`
	DummyBridgesFile      string                 `json:"dummy_bridges_file"`
	TimeDistribution      TimeDistributionConfig `json:"time_distribution"`
	WebApi                WebApiConfig           `json:"web_api"`
	TrustProxy            bool                   `json:"trust_proxy"`
}

type TelegramDistConfig struct {
	Resource             string            `json:"resource"`
	NumBridgesPerRequest int               `json:"num_bridges_per_request"`
	RotationPeriodHours  int               `json:"rotation_period_hours"`
	Token                string            `json:"token"`
	MinUserID            int64             `json:"min_user_id"`
	UpdaterTokens        map[string]string `json:"updater_tokens"`
	StorageDir           string            `json:"storage_dir"`
	ApiAddress           string            `json:"api_address"`
	LoxServerAddress     string            `json:"lox_server_address"`
}

type WebApiConfig struct {
	ApiAddress string `json:"api_address"`
	CertFile   string `json:"cert_file"`
	KeyFile    string `json:"key_file"`
}

type EmailConfig struct {
	Address      string `json:"address"`
	SmtpServer   string `json:"smtp_server"`
	SmtpUsername string `json:"smtp_username"`
	SmtpPassword string `json:"smtp_password"`
	ImapServer   string `json:"imap_server"`
	ImapUsername string `json:"imap_username"`
	ImapPassword string `json:"imap_password"`
}

type TimeDistributionConfig struct {
	NumBridgesPerRequest int    `json:"num_bridges_per_request"`
	RotationPeriodHours  int    `json:"rotation_period_hours"`
	NumPeriods           int    `json:"num_periods"`
	StorageDir           string `json:"storage_dir"`
}

type Updaters struct {
	Gettor GettorUpdater `json:"gettor"`
}

type GettorUpdater struct {
	Github             Github             `json:"github"`
	Gitlab             Gitlab             `json:"gitlab"`
	S3Updaters         []S3Updater        `json:"s3"`
	GoogleDriveUpdater GoogleDriveUpdater `json:"gdrive"`
	MetricsAddress     string             `json:"metrics_address"`
}

type Github struct {
	AuthToken string `json:"auth_token"`
	Owner     string `json:"owner"`
	Repo      string `json:"repo"`
}

type Gitlab struct {
	AuthToken string `json:"auth_token"`
	Owner     string `json:"owner"`
}

type S3Updater struct {
	AccessKey                    string `json:"access_key"`
	AccessSecret                 string `json:"access_secret"`
	SigningMethod                string `json:"signing_method"`
	EndpointUrl                  string `json:"endpoint_url"`
	EndpointRegion               string `json:"endpoint_region"`
	Name                         string `json:"name"`
	Bucket                       string `json:"bucket"`
	NameProceduralGenerationSeed string `json:"name_procedural_generation_seed"`
}

type GoogleDriveUpdater struct {
	AppCredentialPath  string `json:"app_credential_path"`
	UserCredentialPath string `json:"user_credential_path"`
	ParentFolderID     string `json:"parent_folder_id"`
}

type WhatsAppConfig struct {
	SessionFile    string `json:"session_file"`
	MetricsAddress string `json:"metrics_address"`
}

// LoadConfig loads the given JSON configuration file and returns the resulting
// Config configuration object.
func LoadConfig(filename string) (*Config, error) {
	var config Config
	err := config.Set(filename)
	return &config, err
}

// Set loads the given JSON configuration file rewritting the existing config
func (config *Config) Set(filename string) error {
	log.Printf("Attempting to load configuration file at %s.", filename)

	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(content, config); err != nil {
		return err
	}

	config.isIntialized = true
	return nil
}

func (config *Config) String() string {
	return ""
}

// ResourceStreamURL returns the url to connect to the resource stream endpoint
func (bc BackendConfig) ResourceStreamURL() string {
	return bc.urlProto() + bc.WebApi.ApiAddress + bc.ResourceStreamEndpoint
}

// ResourceStreamURL returns the url to connect to the resources endpoint
func (bc BackendConfig) ResourcesURL() string {
	return bc.urlProto() + bc.WebApi.ApiAddress + bc.ResourcesEndpoint
}

// urlProto returns the protocol that should be used to connect to the Api
// if ApiAddress is an IP it will be http otherways will be https
func (bc BackendConfig) urlProto() string {
	host, _, err := net.SplitHostPort(bc.WebApi.ApiAddress)
	if err != nil || net.ParseIP(host) == nil {
		return "https://"
	}
	return "http://"
}
