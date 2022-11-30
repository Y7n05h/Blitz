.PHONY: build deploy clean

TAG?=$(shell git describe --tags --dirty --always)

GO_BUILD=CGO_ENABLED=0 go build -mod=vendor -gcflags="all=-N -l"

build:
	$(GO_BUILD) -o build/blitz cmd/blitz/main.go
	$(GO_BUILD) -o build/blitzd cmd/blitzd/main.go
VERSION=v0.1
image: build
	sudo nerdctl build -t blitz:$(TAG) -f script/Dockerfile .
	sudo nerdctl save -o build/blitz-$(TAG).docker blitz:$(TAG)
deploy:
	kubectl apply -f doc/blitz.yaml

clean:
	rm -rf build
	go mod tidy
	go mod vendor
