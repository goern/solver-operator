# kernel-style V=1 build verbosity
ifeq ("$(origin V)", "command line")
       BUILD_VERBOSE = $(V)
endif

ifeq ($(BUILD_VERBOSE),1)
       Q =
else
       Q = @
endif

VERSION = $(shell git describe --dirty --tags --always)
REPO = github.com/thoth-station/solver-operator
BUILD_PATH = $(REPO)/cmd/manager
PKGS = $(shell go list ./... | grep -v /vendor/)

export CGO_ENABLED:=0

all: format test build/solver-operator

format:
	$(Q)go fmt $(PKGS)

dep:
	$(Q)dep ensure -v

clean:
	$(Q)rm -f build/solver-operator

.PHONY: all test format dep clean

install:
	$(Q)go install $(BUILD_PATH)

release_x86_64 := \
	build/solver-operator-$(VERSION)-x86_64-linux-gnu

release: clean $(release_x86_64) $(release_x86_64:=.asc)

build/solver-operator-%-x86_64-linux-gnu: GOARGS = GOOS=linux GOARCH=amd64

build/%:
	$(Q)$(GOARGS) go build -o $@ $(BUILD_PATH)
	
build/%.asc:
	$(Q){ \
	default_key=$$(gpgconf --list-options gpg | awk -F: '$$1 == "default-key" { gsub(/"/,""); print toupper($$10)}'); \
	git_key=$$(git config --get user.signingkey | awk '{ print toupper($$0) }'); \
	if [ "$${default_key}" = "$${git_key}" ]; then \
		gpg --output $@ --detach-sig build/$*; \
		gpg --verify $@ build/$*; \
	else \
		echo "git and/or gpg are not configured to have default signing key $${default_key}"; \
		exit 1; \
	fi; \
	}

.PHONY: install release_x86_64 release

test: dep

.PHONY: test 