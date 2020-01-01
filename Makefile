# Copyright (c) 2015-2019 Magnus Bäck <magnus@noun.se>

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

# The Docker image to use when building release images.
GOLANG_DOCKER_IMAGE := golang:1.13.5

INSTALL := install

# Installation prefix directory. Could be changed to e.g. /usr or
# /opt/logstash-filter-verifier.
PREFIX := /usr/local

# The name of the executable produced by this makefile.
PROGRAM := logstash-filter-verifier

# List of all GOOS_GOARCH combinations that we should build release
# binaries for. See https://golang.org/doc/install/source#environment
# for all available combinations.
TARGETS := darwin_amd64 linux_386 linux_amd64 windows_386 windows_amd64

VERSION := $(shell git describe --tags --always)

GOCOV         := $(GOBIN)/gocov$(EXEC_SUFFIX)
GOCOV_HTML    := $(GOBIN)/gocov-html$(EXEC_SUFFIX)
GOLANGCI_LINT := $(GOBIN)/golangci-lint$(EXEC_SUFFIX)
GOVVV         := $(GOBIN)/govvv$(EXEC_SUFFIX)
OVERALLS      := $(GOBIN)/overalls$(EXEC_SUFFIX)

GOLANGCI_LINT_VERSION := v1.22.2

.PHONY: all
all: $(PROGRAM)$(EXEC_SUFFIX)

# Depend on this target to force a rebuild every time.
.FORCE:

$(GOCOV):
	go get github.com/axw/gocov/gocov

$(GOCOV_HTML):
	go get github.com/matm/gocov-html

$(GOLANGCI_LINT):
	curl --silent --show-error --location \
	    https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
		| sh -s -- -b $(dir $(GOLANGCI_LINT)) $(GOLANGCI_LINT_VERSION)

$(GOVVV):
	go get github.com/ahmetb/govvv

$(OVERALLS):
	go get github.com/go-playground/overalls

# The Go compiler is fast and pretty good about figuring out what to
# build so we don't try to to outsmart it.
$(PROGRAM)$(EXEC_SUFFIX): .FORCE $(GOVVV)
	govvv build -o $@

.PHONY: check
check: $(GOLANGCI_LINT)
	golangci-lint run

.PHONY: clean
clean:
	rm -f $(PROGRAM)$(EXEC_SUFFIX) $(GOCOV) $(GOCOV_HTML) $(GOLANGCI_LINT) $(GPM) $(OVERALLS)
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
	GOOS="$$(basename $@ .tar.gz | awk -F_ '{print $$3}')" && \
	    GOARCH="$$(basename $@ .tar.gz | awk -F_ '{print $$4}')" && \
	    DISTDIR=dist/$${GOOS}_$${GOARCH} && \
	    if [ $$GOOS = "windows" ] ; then EXEC_SUFFIX=".exe" ; fi && \
	    mkdir -p $$DISTDIR && \
	    cp README.md LICENSE $$DISTDIR && \
	    BINDMOUNTS=$$(echo $$GOPATH | \
	        awk -F: '{ for (i = 1; i<= NF; i++) { printf " -v %s:%s\n", $$i, $$i } }') && \
	    docker run -it --rm $$BINDMOUNTS -w $$(pwd) \
	        -e GOOS=$$GOOS -e GOARCH=$$GOARCH \
	        $(GOLANG_DOCKER_IMAGE) \
	        govvv build -o $$DISTDIR/$(PROGRAM)$$EXEC_SUFFIX && \
	    tar -C $$DISTDIR -zcpf $@ . && \
	    rm -rf $$DISTDIR

.PHONY: test
test: $(GOCOV) $(GOCOV_HTML) $(OVERALLS) $(PROGRAM)$(EXEC_SUFFIX)
	$(OVERALLS) -project=$$(pwd) -covermode=count
	$(GOCOV) convert overalls.coverprofile | $(GOCOV_HTML) > coverage.html
