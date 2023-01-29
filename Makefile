.PHONY: build deploy cleanImage image image-push image-save clean blitz blitzd static-release release
.DEFAULT_GOAL := build

BUILD_DIR?="build"
docker_exec?=docker

REVISION:=$(shell git rev-parse --short HEAD )
BUILD_TIME:=$(shell date --iso-8601=minutes --utc)
VERSION:=$(shell git describe --tags --dirty --always)
AboutInfo=$(addprefix -X blitz/pkg/constant., Version=${VERSION} Revision=${REVISION} BuildTime=${BUILD_TIME})
REGISTRY?=ghcr.io/y7n05h/blitz

GO_BUILD_FLAGS?=-trimpath -buildmode=pie -mod=readonly -modcacherw
RELEASE_GO_BUILD_FLAGS?=${GO_BUILD_FLAGS}
DEBUG_GO_BUILD_FLAGS?=${GO_BUILD_FLAGS} -gcflags="all=-N -l"
EXTLDFLAGS?=${LDFLAGS} -z now
GO_LDFLAGS?= -linkmode external -extldflags \"${EXTLDFLAGS}\" ${AboutInfo}
RELEASE_GO_LDFLAGS?= -s -w ${GO_LDFLAGS} -buildid=
DEBUG_GO_LDFLAGS?=${GO_LDFLAGS}

ifneq "${STATIC}" ""
	EXTLDFLAGS+= -static
endif

ifeq "${DEBUG}" ""
	GO_BUILD=go build ${RELEASE_GO_BUILD_FLAGS} -ldflags "${RELEASE_GO_LDFLAGS}"
else
	GO_BUILD=go build ${DEBUG_GO_BUILD_FLAGS} -ldflags "${DEBUG_GO_LDFLAGS}"
endif

blitz:
	$(GO_BUILD) -o ${BUILD_DIR}/blitz cmd/blitz/main.go
blitzd:
	$(GO_BUILD) -o ${BUILD_DIR}/blitzd cmd/blitzd/main.go

build: blitz blitzd

image: cleanImage build
	$(docker_exec) build -t ${REGISTRY}:${VERSION} -f script/Dockerfile .
image-push: image
	$(docker_exec) push ${REGISTRY}:${VERSION}
image-save: image
	$(docker_exec) save -o build/blitz-${VERSION}.docker blitz:${VERSION}
deploy:
	kubectl apply -f doc/blitz.yaml
cleanImage:
	rm -rf build/*.docker
clean:
	rm -rf build
	go mod tidy
	go mod vendor
release: image
static-release: EXTLDFLAGS+= -static
static-release: GO_BUILD=go build ${DEBUG_GO_BUILD_FLAGS} -ldflags "${DEBUG_GO_LDFLAGS}"
static-release: release
