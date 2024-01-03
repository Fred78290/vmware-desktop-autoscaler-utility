ALL_ARCH = amd64 arm64

.EXPORT_ALL_VARIABLES:

all:
	make -e GOOS=linux -e TAG=${TAG} $(addprefix build-arch-,$(ALL_ARCH))
	make -e GOOS=darwin -e TAG=${TAG} $(addprefix build-arch-,$(ALL_ARCH))

VERSION_MAJOR ?= 0
VERSION_MINOR ?= 1
VERSION_BUILD ?= 1
TAG?=v$(VERSION_MAJOR).$(VERSION_MINOR).$(VERSION_BUILD)
FLAGS=
ENVVAR=CGO_ENABLED=0
LDFLAGS?=-s
GOOS?=$(shell go env GOOS)
GOARCH?=$(shell go env GOARCH)
BUILD_DATE?=`date +%Y-%m-%dT%H:%M:%SZ`

deps:
	go mod vendor

build: $(addprefix build-arch-,$(ALL_ARCH))

build-arch-%: deps clean-arch-%
	$(ENVVAR) GOOS=$(GOOS) GOARCH=$* BUILD_DATE=`date +%Y-%m-%dT%H:%M:%SZ` go build -buildvcs=false -ldflags="-X github.com/Fred78290/vmware-desktop-autoscaler-utility/version.VERSION=$(TAG) -X github.com/Fred78290/vmware-desktop-autoscaler-utility/version.BUILD_DATE=$(BUILD_DATE) ${LDFLAGS}" -a -o out/$(GOOS)/$*/vmware-desktop-autoscaler-utility

test-unit: clean build
	bash ./scripts/run-tests.sh

clean: $(addprefix clean-arch-,$(ALL_ARCH))

clean-arch-%:
	rm -f ./out/$(GOOS)/$*/vmware-desktop-autoscaler-utility

docker-builder:
	docker build -t vmware-desktop-autoscaler-utility-builder ./builder

build-in-docker: $(addprefix build-in-docker-arch-,$(ALL_ARCH))

build-in-docker-arch-%: clean-arch-% docker-builder
	docker run --rm -v `pwd`:/gopath/src/github.com/Fred78290/vmware-desktop-autoscaler-utility/ vmware-desktop-autoscaler-utility-builder:latest bash \
		-c 'cd /gopath/src/github.com/Fred78290/vmware-desktop-autoscaler-utility \
		&& make -e GOOS=linux -e TAG=${TAG} build-arch-$* \
		&& make -e GOOS=darwin -e TAG=${TAG} build-arch-$*'

container: $(addprefix container-arch-,$(ALL_ARCH))

container-arch-%: build-in-docker-arch-%
	@echo "Full in-docker image ${TAG}-$* completed"

test-in-docker: docker-builder
	docker run --rm -v `pwd`:/gopath/src/github.com/Fred78290/vmware-desktop-autoscaler-utility/ vmware-desktop-autoscaler-utility-builder:latest bash \
		-c 'cd /gopath/src/github.com/Fred78290/vmware-desktop-autoscaler-utility && bash ./scripts/run-tests.sh'

install: build-arch-$(GOARCH)
	$(ENVVAR) cp out/$(GOOS)/$(GOARCH)/vmware-desktop-autoscaler-utility /usr/local/bin
	vmware-desktop-autoscaler-utility version

.PHONY: all build test-unit clean docker-builder build-in-docker install
