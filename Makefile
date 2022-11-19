.PHONY: build deploy clean

TAG?=$(shell git describe --tags --dirty --always)

GO_BUILD=CGO_ENABLED=0 go build -mod=vendor -gcflags="all=-N -l"

build:
	$(GO_BUILD) -o build/tcni cmd/tcni/main.go
	$(GO_BUILD) -o build/tcnid cmd/tcnid/main.go
VERSION=v0.1
image: build
	sudo nerdctl build -t tcni:$(TAG) -f script/Dockerfile .
	sudo nerdctl save -o build/tcni-$(TAG).docker tcni:$(TAG)
deploy:
	kubectl apply -f doc/tcni.yaml

clean:
	rm -rf build
	go mod tidy
	go mod vendor
