.PHONY: build deploy cleanImage clean blitz blitzd

BUILD_DIR?="build"
docker_exec?=docker

REVISION:=$(shell git rev-parse --short HEAD )
BUILD_TIME:=$(shell date --iso-8601=minutes --utc)
VERSION:=$(shell git describe --tags --dirty --always)
AboutInfo=$(addprefix -X blitz/pkg/constant., Version=${VERSION} Revision=${REVISION} BuildTime=${BUILD_TIME})

GO_BUILD_FLAGS?=-trimpath -buildmode=pie -mod=readonly -modcacherw
RELEASE_GO_BUILD_FLAGS?=${GO_BUILD_FLAGS}
DEBUG_GO_BUILD_FLAGS?=${GO_BUILD_FLAGS} -gcflags="all=-N -l"

GO_LDFLAGS?= -linkmode external -extldflags \"${LDFLAGS}\" ${AboutInfo}
RELEASE_GO_LDFLAGS?= -s -w ${GO_LDFLAGS}
DEBUG_GO_LDFLAGS?=${GO_LDFLAGS}

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
	$(docker_exec) build -t blitz:${VERSION} -f script/Dockerfile .
	$(docker_exec) save -o build/blitz-${VERSION}.docker blitz:${VERSION}
deploy:
	kubectl apply -f doc/blitz.yaml
cleanImage:
	rm -rf build/*.docker
clean:
	rm -rf build
	go mod tidy
	go mod vendor
