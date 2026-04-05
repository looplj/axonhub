.PHONY: build clean package

BINARY_NAME=axonclaw
ZIP_NAME=axonclaw.zip

VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME?=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

LDFLAGS=-ldflags "\
	-X github.com/looplj/axonhub/cmd/axonclaw/build.Version=$(VERSION) \
	-X github.com/looplj/axonhub/cmd/axonclaw/build.BuildTime=$(BUILD_TIME) \
	-X github.com/looplj/axonhub/cmd/axonclaw/build.GitCommit=$(GIT_COMMIT) \
	-s -w"

build:
	go build $(LDFLAGS) -o $(BINARY_NAME) .

package: build
	@COPYFILE_DISABLE=1 zip -r $(ZIP_NAME) $(BINARY_NAME) *.sh *.bat *.ps1 -x "*.DS_Store" -x "__MACOSX/*"

clean:
	rm -f $(BINARY_NAME) $(ZIP_NAME)
