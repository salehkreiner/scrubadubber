# Scrubadubber build helpers.
#
# Windows users without `make` can run the underlying `go` commands directly
# (see README). CI invokes the platform builds explicitly in .github/workflows.

APP        := scrubadubber
VERSION    ?= dev
MODULE     := github.com/salehkreiner/scrubadubber
LDFLAGS    := -X $(MODULE)/internal/version.Version=$(VERSION)
GOVERSIONINFO := go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest

.PHONY: all test vet icons clean tray installer syso mac-app mac-dmg

all: test vet

test:
	go test ./... -count=1

vet:
	go vet ./...

# Regenerate the embedded tray icons.
icons:
	go run ./assets/gen

# --- Windows -------------------------------------------------------------
# Embed the UAC (asInvoker) manifest + icon + version into each command, then
# build. Run on a Windows host.
syso:
	$(GOVERSIONINFO) -o cmd/scrubadubber/resource.syso packaging/windows/versioninfo.json
	$(GOVERSIONINFO) -o cmd/installer/resource.syso packaging/windows/versioninfo.json

tray:
	go build -trimpath -ldflags "$(LDFLAGS) -H windowsgui" -o dist/$(APP)_windows_amd64.exe ./cmd/scrubadubber

installer:
	go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(APP)-setup.exe ./cmd/installer

# --- macOS ---------------------------------------------------------------
mac-app:
	bash packaging/macos/build-app.sh $(VERSION)

mac-dmg: mac-app
	bash packaging/macos/build-dmg.sh

clean:
	rm -rf dist cmd/scrubadubber/resource.syso cmd/installer/resource.syso
