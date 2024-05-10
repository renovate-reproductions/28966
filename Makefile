descriptors:
	go run scripts/mkdescriptors/main.go descriptors

build:
	CGO_ENABLED=0 go build ./cmd/backend
	CGO_ENABLED=0 go build ./cmd/distributors
	CGO_ENABLED=0 go build ./cmd/updaters

translations:
	@goi18n extract -format json  -outdir pkg/locales
	xspreak  -p pkg/locales -f json -k translated -t "./pkg/presentation/distributors/https/embedded/*.html"
	jq -s '.[0] * .[1]'  pkg/locales/messages.json pkg/locales/active.en.json > pkg/locales/active_merged.en.json
	rm pkg/locales/messages.json pkg/locales/active.en.json
	mv pkg/locales/active_merged.en.json pkg/locales/active.en.json

fetch_translations:
	mv pkg/locales/active.en.json pkg/locales/active.en.json.tmp
	rm pkg/locales/active.*.json
	mv pkg/locales/active.en.json.tmp pkg/locales/active.en.json
	./scripts/fetch_translations.py pkg/locales
