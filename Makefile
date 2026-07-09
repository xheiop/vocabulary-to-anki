BINARY      := vocab2anki
BIN_DIR     := $(HOME)/.local/bin
BIN_PATH    := $(BIN_DIR)/$(BINARY)
CONFIG_SRC  := config.toml
CONFIG_DST  := $(HOME)/.config/vocab2anki/config.toml
PLIST_LABEL := com.vocab2anki
PLIST_DST   := $(HOME)/Library/LaunchAgents/$(PLIST_LABEL).plist
WORKDIR     := $(HOME)/.config/vocab2anki

# Load ANTHROPIC_API_KEY from .env if present (for `make run` / `make install`).
ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: build run test tidy fmt install uninstall logs

build:
	go build -o bin/$(BINARY) ./cmd/vocab2anki

run: build
	./bin/$(BINARY) -config $(CONFIG_SRC)

test:
	go test ./...

tidy:
	go mod tidy

fmt:
	gofmt -w .

# Build, install the binary + config, render the launchd plist with real paths
# (and the API key if set — only needed for the "api" backend), then load it.
install: build
	@test -n "$(ANTHROPIC_API_KEY)" || echo "note: ANTHROPIC_API_KEY not set (fine for the default CLI backend)"
	install -d $(BIN_DIR) $(WORKDIR)
	install -m 0755 bin/$(BINARY) $(BIN_PATH)
	rm -f $(CONFIG_DST)
	install -m 0644 $(CONFIG_SRC) $(CONFIG_DST)
	install -d $(HOME)/Library/LaunchAgents
	sed -e 's|__BIN__|$(BIN_PATH)|g' \
	    -e 's|__CONFIG__|$(CONFIG_DST)|g' \
	    -e 's|__WORKDIR__|$(WORKDIR)|g' \
	    -e 's|__HOME__|$(HOME)|g' \
	    -e 's|__ANTHROPIC_API_KEY__|$(ANTHROPIC_API_KEY)|g' \
	    launchd/$(PLIST_LABEL).plist > $(PLIST_DST)
	chmod 0600 $(PLIST_DST)
	-launchctl unload $(PLIST_DST) 2>/dev/null
	launchctl load $(PLIST_DST)
	@echo "Installed and loaded $(PLIST_LABEL). Logs: ~/Library/Logs/vocab2anki.log"

uninstall:
	-launchctl unload $(PLIST_DST) 2>/dev/null
	-rm -f $(PLIST_DST) $(BIN_PATH)
	@echo "Uninstalled $(PLIST_LABEL) (config at $(CONFIG_DST) left in place)."

logs:
	tail -f $(HOME)/Library/Logs/vocab2anki.log
