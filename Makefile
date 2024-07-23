# Copyright (c) 2015-2020 Magnus Bäck <magnus@noun.se>

export GOBIN := $(shell pwd)/bin
export PATH := $(GOBIN):$(PATH)

# Installation root directory. Should be left alone except for
# e.g. package installations. If you want to control the installation
# directory for normal use you should modify PREFIX instead.
DESTDIR :=

ifeq ($(OS),Windows_NT)
EXEC_SUFFIX := .exe
OS_NAME := Windows
else
EXEC_SUFFIX :=
OS_NAME := $(shell uname -s)
endif

INSTALL := install

# Installation prefix directory. Could be changed to e.g. /usr or
# /opt/logstash-filter-verifier.
PREFIX := /usr/local

# The name of the executable produced by this makefile.
PROGRAM := logstash-filter-verifier

# List of all GOOS_GOARCH combinations that we should build release
# binaries for. See https://golang.org/doc/install/source#environment
# for all available combinations.
TARGETS := darwin_amd64 linux_386 linux_amd64

VERSION := $(shell git describe --tags --always)

GOCOV              := $(GOBIN)/gocov$(EXEC_SUFFIX)
GOCOV_HTML         := $(GOBIN)/gocov-html$(EXEC_SUFFIX)
GOLANGCI_LINT      := $(GOBIN)/golangci-lint$(EXEC_SUFFIX)
GOVVV              := $(GOBIN)/govvv$(EXEC_SUFFIX)
OVERALLS           := $(GOBIN)/overalls$(EXEC_SUFFIX)
PROTOC_GEN_GO      := $(GOBIN)/protoc-gen-go$(EXEC_SUFFIX)
PROTOC_GEN_GO_GRPC := $(GOBIN)/protoc-gen-go-grpc$(EXEC_SUFFIX)
MOQ                := $(GOBIN)/moq$(EXEC_SUFFIX)

GOLANGCI_LINT_VERSION := v1.59.0

.PHONY: all
all: $(PROGRAM)$(EXEC_SUFFIX)

# Depend on this target to force a rebuild every time.
.FORCE:

$(GOCOV):
	go install github.com/axw/gocov/gocov

$(GOCOV_HTML):
	go install github.com/matm/gocov-html

$(GOLANGCI_LINT):
	curl --silent --show-error --location \
	    https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
		| sh -s -- -b $(dir $(GOLANGCI_LINT)) $(GOLANGCI_LINT_VERSION)

$(GOVVV):
	go install github.com/ahmetb/govvv

$(OVERALLS):
	go install github.com/go-playground/overalls

$(PROTOC_GEN_GO):
	go install google.golang.org/protobuf/cmd/protoc-gen-go

$(PROTOC_GEN_GO_GRPC):
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc

$(MOQ):
	go install github.com/matryer/moq

# The Go compiler is fast and pretty good about figuring out what to
# build so we don't try to to outsmart it.
$(PROGRAM)$(EXEC_SUFFIX): gogenerate .FORCE $(GOVVV)
	govvv build -o $@

.PHONY: gogenerate
gogenerate: $(MOQ) $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC)
	go generate ./...

.PHONY: check
check: $(GOLANGCI_LINT)
	golangci-lint run

.PHONY: checktidy
checktidy:
	go mod tidy && git diff --exit-code -- go.mod go.sum

.PHONY: clean
clean:
	rm -f $(PROGRAM)$(EXEC_SUFFIX)
	rm -rf $(GOBIN)
	rm -rf dist

.PHONY: install
install: $(DESTDIR)$(PREFIX)/bin/$(PROGRAM)$(EXEC_SUFFIX)

$(DESTDIR)$(PREFIX)/bin/%: %
	mkdir -p $(dir $@)
ifeq ($(OS_NAME),Darwin)
	$(INSTALL) -m 0755 $< $@
else
	$(INSTALL) -m 0755 --strip $< $@
endif

.PHONY: release-tarballs
release-tarballs: dist/$(PROGRAM)_$(VERSION).tar.gz \
    $(addsuffix .tar.gz,$(addprefix dist/$(PROGRAM)_$(VERSION)_,$(TARGETS)))

dist/$(PROGRAM)_$(VERSION).tar.gz:
	mkdir -p $(dir $@)
	git archive --output=$@ HEAD

dist/$(PROGRAM)_$(VERSION)_%.tar.gz: $(GOVVV)
	mkdir -p $(dir $@)
	export GOOS="$$(basename $@ .tar.gz | awk -F_ '{print $$3}')" && \
	    export GOARCH="$$(basename $@ .tar.gz | awk -F_ '{print $$4}')" && \
	    DISTDIR=dist/$${GOOS}_$${GOARCH} && \
	    if [ $$GOOS = "windows" ] ; then EXEC_SUFFIX=".exe" ; fi && \
	    mkdir -p $$DISTDIR && \
	    cp CHANGELOG.md LICENSE README.md $$DISTDIR && \
	    bin/govvv build -o $$DISTDIR/$(PROGRAM)$$EXEC_SUFFIX && \
	    tar -C $$DISTDIR -zcpf $@ . && \
	    rm -rf $$DISTDIR

.PHONY: test
test: $(GOCOV) $(GOCOV_HTML) $(OVERALLS) $(PROGRAM)$(EXEC_SUFFIX)
	$(OVERALLS) -project=$$(pwd) -covermode=count
	$(GOCOV) convert overalls.coverprofile | $(GOCOV_HTML) > coverage.html
