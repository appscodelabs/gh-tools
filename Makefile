SHELL := /bin/bash
BIN := gh-tools
CGO_ENV ?= CGO_ENABLED=0
PKG := github.com/appscodelabs/$(BIN)
UID := $(shell id -u $$USER)

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
GOARM ?= $(shell go env GOARM)
GOPATH = $(shell go env GOPATH)
REPO_ROOT := $(GOPATH)/src/$(PKG)

platforms := linux/amd64 linux/arm64 linux/arm/7 linux/arm/6 windows/amd64 darwin/amd64

# metadata
commit_hash := $(shell git rev-parse --verify HEAD)
git_branch := $(shell git rev-parse --abbrev-ref HEAD)
git_tag := $(shell git describe --exact-match --abbrev=0 2>/dev/null || echo "")
commit_timestamp := $(shell date --date="@$$(git show -s --format=%ct)" --utc +%FT%T)
build_timestamp := $(shell date --utc +%FT%T)
build_host:= $(shell hostname)
build_host_os:= $(shell go env GOHOSTOS)
build_host_arch:= $(shell go env GOHOSTARCH)
version_strategy := commit_hash
version := $(shell git describe --tags --always --dirty)

# compiler flags
linker_opts := -X main.GitTag=$(git_tag) -X main.CommitHash=$(commit_hash) -X main.CommitTimestamp=$(commit_timestamp) \
	-X main.VersionStrategy=$(version_strategy) -X main.Version=$(version) -X main.GitBranch=$(git_branch) \
	-X main.Os=$(GOOS) -X main.Arch=$(GOARCH)

ifeq ($(CGO_ENV),CGO_ENABLED=1)
	CGO := -a -installsuffix cgo
	linker_opts += -linkmode external -extldflags -static -w
endif

ifdef git_tag
	version := $(git_tag)
	version_strategy := tag
else
	ifneq ($(git_branch),$(or master, HEAD))
		ifeq (,$(findstring release-,$(git_branch)))
			version := $(git_branch)
			version_strategy := branch
			linker_opts += -X main.BuildTimestamp=$(build_timestamp) -X main.BuildHost=$(build_host) \
						   -X main.BuildHostOS=$(build_host_os) -X main.BuildHostArch=$(build_host_arch)
		endif
	endif
endif
ldflags :=-ldflags '$(linker_opts)'

SOURCES := $(shell find . -name "*.go")

# build locally
dist/$(BIN)/local/$(BIN)-$(GOOS)-$(GOARCH)$(GOARM): $(SOURCES)
	@cowsay -f tux building binary $(BIN)-$(GOOS)-$(GOARCH)$(GOARM)
	GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) $(CGO_ENV) \
		go build -o dist/$(BIN)/local/$(BIN)-$(GOOS)-$(GOARCH)$(GOARM) \
		$(CGO) $(ldflags) *.go

# build inside docker
dist/$(BIN)/$(BIN)-$(GOOS)-$(GOARCH)$(GOARM): $(SOURCES)
	@cowsay -f tux building binary $(BIN)-$(GOOS)-$(GOARCH)$(GOARM) inside docker
	docker run --rm -u $(UID) -v /tmp:/.cache -v $$(pwd):/go/src/$(PKG) -w /go/src/$(PKG) \
		-e $(CGO_ENV) golang:1.10.0 env GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) $(CGO_ENV) \
		go build -o dist/$(BIN)/$(BIN)-$(GOOS)-$(GOARCH)$(GOARM) \
		$(CGO) $(ldflags) *.go

# nfpm
dist/$(BIN)/package/$(BIN)-$(GOOS)-$(GOARCH)$(GOARM).%: build-docker
	@cowsay -f tux creating package $(BIN)-$(GOOS)-$(GOARCH)$(GOARM).$*
	@mkdir -p dist/package
	docker run --rm -v $(REPO_ROOT):/go/src/$(PKG) -w /go/src/$(PKG) tahsin/releaser:latest /bin/bash -c \
		"sed -i 's/amd64/$(GOARCH)$(GOARM)/' /nfpm.yaml; \
		sed -i 's/linux/$(GOOS)/' /nfpm.yaml; \
		sed -i '4s/1.0.0/$(version)/' /nfpm.yaml; \
		sed -i 's/BIN/$(BIN)/g' /nfpm.yaml; \
		nfpm pkg --target /go/src/$(PKG)/dist/package/$(BIN)-$(GOOS)-$(GOARCH)$(GOARM).$* -f /nfpm.yaml"

build-local: dist/$(BIN)/local/$(BIN)-$(GOOS)-$(GOARCH)$(GOARM)
build-docker: dist/$(BIN)/$(BIN)-$(GOOS)-$(GOARCH)$(GOARM)

all-build-%:
	@for platform in $(platforms); do \
		IFS='/' read -r -a array <<< $$platform; \
		GOOS=$${array[0]}; GOARCH=$${array[1]}; GOARM=$${array[2]}; \
		$(MAKE) --no-print-directory GOOS=$$GOOS GOARCH=$$GOARCH GOARM=$$GOARM build-$*; \
	done

package-%:
	@mkdir -p dist/$(BIN)/package
	$(MAKE) --no-print-directory dist/$(BIN)/package/$(BIN)-$(GOOS)-$(GOARCH)$(GOARM).$*

linux_package_platforms := linux/amd64 linux/arm64 linux/arm/7 linux/arm/6
all-package:
	@for platform in $(linux_package_platforms); do \
		IFS='/' read -r -a array <<< $$platform; \
		GOOS=$${array[0]}; GOARCH=$${array[1]}; GOARM=$${array[2]}; \
		$(MAKE) --no-print-directory GOOS=$$GOOS GOARCH=$$GOARCH GOARM=$$GOARM package-deb; \
		$(MAKE) --no-print-directory GOOS=$$GOOS GOARCH=$$GOARCH GOARM=$$GOARM package-rpm; \
	done

.phony: build-docker build-local all-build-% package-% all-package
