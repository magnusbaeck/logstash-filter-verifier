# Copyright (c) 2015 Magnus Bäck <magnus@noun.se>

# Installation root directory. Should be left alone except for
# e.g. package installations. If you want to control the installation
# directory for normal use you should modify PREFIX instead.
DEST_DIR :=

# Installation prefix directory. Could be changed to e.g. /usr or
# /opt/logstash-filter-verifier.
PREFIX := /usr/local

.PHONY: all
all: logstash-filter-verifier

# Depend on this target to force a rebuild every time.
.FORCE:

# The Go compiler is fast and pretty good about figuring out what to
# build so we don't try to to outsmart it.
logstash-filter-verifier: .FORCE
	go get
	go build

.PHONY: clean
clean:
	rm -f logstash-filter-verifier

.PHONY: install
install: logstash-filter-verifier
	mkdir -p $(DESTDIR)$(PREFIX)/bin
	install -m 0755 --strip logstash-filter-verifier $(DESTDIR)$(PREFIX)/bin
