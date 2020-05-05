# Logstash Filter Verifier

[![Travis](https://travis-ci.org/magnusbaeck/logstash-filter-verifier.svg?branch=master)](https://travis-ci.org/magnusbaeck/logstash-filter-verifier)
[![GoReportCard](http://goreportcard.com/badge/magnusbaeck/logstash-filter-verifier)](http://goreportcard.com/report/magnusbaeck/logstash-filter-verifier)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://raw.githubusercontent.com/magnusbaeck/logstash-filter-verifier/master/LICENSE)

* [Introduction](#introduction)
* [Installing](#installing)
* [Examples](#examples)
  * [Syslog messages](#syslog-messages)
  * [JSON messages](#json-messages)
* [Test case file reference](#test-case-file-reference)
* [Migrating to the current test case file format](#migrating-to-the-current-test-case-file-format)
* [Notes](#notes)
  * [The \-\-sockets flag](#the---sockets-flag)
  * [The \-\-logstash\-arg flag](#the---logstash-arg-flag)
  * [Logstash compatibility](#logstash-compatibility)
  * [Windows compatibility](#windows-compatibility)
* [Known limitations and future work](#known-limitations-and-future-work)
* [License](#license)


## Introduction

The [Logstash](https://www.elastic.co/products/logstash) program for
collecting and processing logs from is popular and commonly used to
process e.g. syslog messages and HTTP logs.

Apart from ingesting log events and sending them to one or more
destinations it can transform the events in various ways, including
extracting discrete fields from flat blocks of text, joining multiple
physical lines into singular logical events, parsing JSON and XML, and
deleting unwanted events. It uses its own domain-specific
configuration language to describe both inputs, outputs, and the
filters that should be applied to events.

Writing the filter configurations necessary to parse events isn't
difficult for someone with basic programming skills, but verifying
that the filters do what you expect can be tedious; especially when
you tweak existing filters and want to make sure that all kinds of
logs will continue to be processed as before. If you get something
wrong you might have millions of incorrectly parsed events before you
realize your mistake.

This is where Logstash Filter Verifier comes in. It lets you define
test case files containing lines of input together with the expected
output from Logstash. Pass one of more such test case files to
Logstash Filter Verifier together with all of your Logstash filter
configuration files and it'll run Logstash for you and verify that
Logstash actually returns what you expect.

Before you can run Logstash Filter Verifier you need to install
it. After covering that, let's start with a simple example and follow
up with reference documentation.


## Installing

All releases of Logstash Filter Verifier are published in binary form
for the most common platforms at
[github.com/magnusbaeck/logstash-filter-verifier/releases](https://github.com/magnusbaeck/logstash-filter-verifier/releases).

If you need to run the program on other platforms or if you want to
modify the program yourself you can build and use it on any platform
for which a recent [Go](https://golang.org/) compiler is
available. Pretty much any platform where Logstash runs should be
fine, including Windows.

Many Linux distributions make some version of the Go compiler easily
installable, but otherwise you can [download and install the latest
version](https://golang.org/dl/). The source code is written to use
[Go modules](https://github.com/golang/go/wiki/Modules) for dependency
management and it seems you need at least Go 1.13.

To just build an executable file you don't need anything but the Go
compiler; just clone the Logstash Filter Verifier repository and run
`go build` from the root directory of the cloned repostiory. If
successful you'll find an executable in the current directory.

One drawback of this is that the program won't get stamped with the
correct version number, so `logstash-filter-verifier --version` will
say "unknown"). To address this and make it easy to run tests and
static checks you need GNU make and other GNU tools.

The makefile can also be used to install Logstash Filter Verifier
centrally, by default in /usr/local/bin but you can change that by
modifying the PREFIX variable. For example, to install it in $HOME/bin
(which is probably in your shell's path) you can issue the following
command:

    $ make install PREFIX=$HOME


## Examples

The examples that follow build upon each other and do not only show
how to use Logstash Filter Verifier to test that particular kind of
log. They also highlight how to deal with different features in logs.


### Syslog messages

Logstash is often used to parse syslog messages, so let's use that as
a first example.

Test case files are in JSON or YAML format and contain a single object
with about a handful of supported properties.

Sample with JSON format:
```json
{
  "fields": {
    "type": "syslog"
  },
  "testcases": [
    {
      "input": [
        "Oct  6 20:55:29 myhost myprogram[31993]: This is a test message"
      ],
      "expected": [
        {
          "@timestamp": "2015-10-06T20:55:29.000Z",
          "host": "myhost",
          "message": "This is a test message",
          "pid": 31993,
          "program": "myprogram",
          "type": "syslog"
        }
      ]
    }
  ]
}
```

Sample with YAML format:
```yaml
fields:
  type: "syslog"
testcases:
  - input:
      - "Oct  6 20:55:29 myhost myprogram[31993]: This is a test message"
    expected:
      - "@timestamp": "2015-10-06T20:55:29.000Z"
        host: "myhost"
        message: "This is a test message"
        pid: 31993
        program: "myprogram"
        type: "syslog"
```

Most Logstash configurations contain filters for multiple kinds of
logs and uses conditions on field values to select which filters to
apply. Those field values are typically set in the input plugins. To
make Logstash treat the test events correctly we can "inject"
additional field values to make the test events look like the real
events to Logstash. In this example, `fields.type` is set to "syslog"
which means that the input events in the test cases in this file will
have that in their `type` field when they're passed to Logstash.

Next, in `input`, we define a single test string that we want to feed
through Logstash, and the `expected` array contains a one-element
array with the event we expect Logstash to emit for the given input.

The `testcases` array can contain multiple objects with `input` and
`expected` keys. For example, if we change the example above to

```yaml
fields:
  type: "syslog"
testcases:
  - input:
      - "Oct  6 20:55:29 myhost myprogram[31993]: This is a test message"
    expected:
      - "@timestamp": "2015-10-06T20:55:29.000Z"
        host: "myhost"
        message: "This is a test message"
        pid: 31993
        program: "myprogram"
        type: "syslog"
  - input:
      - "Oct  6 20:55:29 myhost myprogram: This is a test message"
    expected:
      - "@timestamp": "2015-10-06T20:55:29.000Z"
        host: "myhost"
        message: "This is a test message"
        program: "myprogram"
        type: "syslog"
```

we also test syslog messages that lack the bracketed pid after the
program name.

Note that UTC is the assumed timezone for input events to avoid
different behavior depending on the timezone of the machine where
Logstash Filter Verifier happens to run. This won't affect time
formats that include a timezone.

This command will run this test case file through Logstash Filter
Verifier (replace all "path/to" with the actual paths to the files,
obviously):

    $ path/to/logstash-filter-verifier path/to/syslog.json path/to/filters

If the test is successful, Logstash Filter Verifier will terminate
with a zero exit code and (almost) no output. If the test fails it'll
run `diff -u` (or some other command if you use the `--diff-command`
flag) to compare the pretty-printed JSON representation of the
expected and actual events.

The actual event emitted by Logstash will contain a `@version` field,
but since that field isn't interesting it's ignored by default when
reading the actual event. Hence we don't need to include it in the
expected event either. Additional fields can be ignored with the
`ignore` array property in the test case file (see details below).


### JSON messages

I always prefer to configure applications to emit JSON objects
whenever possible so that I don't have to write complex and/or
ambiguous grok expressions. Here's an example:

```json
{"message": "This is a test message", "client": "127.0.0.1", "host": "myhost", "time": "2015-10-06T20:55:29Z"}
```

When you feed events like this to Logstash it's likely that the
input used will have its codec set to "json_lines". This is something we
should mimic on the Logstash Filter Verifier side too. Use `codec` for
that:

Sample with JSON format:
```json
{
  "fields": {
    "type": "app"
  },
  "codec": "json_lines",
  "ignore": ["host"],
  "testcases": [
    {
      "input": [
        "{\"message\": \"This is a test message\", \"client\": \"127.0.0.1\", \"time\": \"2015-10-06T20:55:29Z\"}"
      ],
      "expected": [
        {
          "@timestamp": "2015-10-06T20:55:29.000Z",
          "client": "localhost",
          "clientip": "127.0.0.1",
          "message": "This is a test message",
          "type": "app"
        }
      ]
    }
  ]
}
```

Sample with YAML format:
```yaml
fields:
  type: "app"
codec: "json_lines"
ignore:
  - "host"
testcases:
  - input:
      - >
        {
          "message": "This is a test message",
          "client": "127.0.0.1",
          "time": "2015-10-06T20:55:29Z"
        }
    expected:
      - "@timestamp": "2015-10-06T20:55:29.000Z"
        client: "localhost"
        clientip: "127.0.0.1"
        message: "This is a test message"
        type: "app"
```

There are a few points to be made here:

* The double quotes inside the string must be escaped when using JSON format.
  YAML files sometimes require quoting too; for example if the value starts
  with `[` or `{` or if a numeric value should be forced to be parsed as a
  string.
* Together with the lack of a need to escape double quotes inside JSON
  strings, the use of `>` to create folded lines in the YAML representation
  makes the input JSON much easier to read.
* The filters being tested here use Logstash's [dns
  filter](https://www.elastic.co/guide/en/logstash/current/plugins-filters-dns.html)
  to transform the IP address in the `client` field into a hostname
  and copy the original IP address into the `clientip` field. To avoid
  future problems and flaky tests, pick a hostname or IP address for
  the test case that will always resolve to the same thing. As in this
  example, localhost and 127.0.0.1 should be safe picks.
* If the input event doesn't contain a `host` field, Logstash will add
  such a field containing the name of the current host. To avoid test
  cases that behave differently depending on the host where they're
  run, we ignore that field with the `ignore` property.


## Test case file reference

Test case files are JSON files containing a single object. That object
may have the following properties:

* `codec`: A string value naming the Logstash codec that should be
  used when events are read. This is normally "line" or "json_lines".
* `fields`: An object containing the fields that all input messages
  should have. This is vital since filters typically are configured
  based on the event's type and/or tags. Scalar values (strings,
  numbers, and booleans) are supported, as are objects (containing
  scalars, arrays and nested objects), arrays of scalars and nested arrays.
  The only combination which is not allowed are objects within arrays.
  A shorthand for defining nested fields is to use the Logstash's field
  reference syntax (`[field][subfield]`), i.e.
  `fields: {"[log][file][path]": "/tmp/test.log"}` is equivalent to
  `fields: {"log": {"file": {"path": "/tmp/test.log"}}}`.
* `ignore`: An array with the names of the fields that should be
  removed from the events that Logstash emit. This is for example
  useful for dynamically generated fields whose contents can't be
  predicted and hardwired into the test case file. If you need to exclude
  individual subfields you can use Logstash's field reference syntax,
  i.e. `[log][file][path]` will exclude that field but keep other subfields
  of `log` like e.g. `[log][level]` and `[log][file][line]`.
* `testcases`: An array of test case objects, each having the following
  contents:
  * `input`: An array with the lines of input (each line being a string)
    that should be fed to the Logstash process. If you use `json_lines` codec
    you can use Logstash's syntax reference syntax for fields in the JSON
    object, making
    `{"message": "my message", "[log][file][path]": "/tmp/test.log"}`
    equivalent to
    `{"message": "my message", "log": {"file": {"path": "/tmp/test.log"}}}`.
  * `expected`: An array of JSON objects with the events to be
    expected. They will be compared to the actual events produced by the
    Logstash process.
  * `description`: An optional textual description of the test case, e.g.
    useful as documentation. This text will be included in the program's
    progress messages.


## Migrating to the current test case file format

Originally the `input` and `expected` configuration keys were at the
top level of the test case file. They were later moved into the
`testcases` key but the old configuration format is still supported.

To migrate test case files from the old to the new file format the
following command using [jq](https://stedolan.github.io/jq/) can be
used (run it in the directory containing the test case files):

```
for f in *.json ; do
    jq '{ codec, fields, ignore, testcases:[[.input[]], [.expected[]]] | transpose | map({input: [.[0]], expected: [.[1]]})} | with_entries(select(.value != null))' $f > $f.migrated && mv $f.migrated $f
done
```

This command only works for test case files where there's a one-to-one
mapping between the elements of the `input` array and the elements of
the `expected` array. If you e.g. have drop and/or split filters in
your Logstash configuration you'll have to patch the converted test
case file by hand afterwards.


## Notes

### The `--sockets` flag

The command line flag `--sockets` allows to use unix domain sockets instead of
stdin to send the input to Logstash. The advantage of this approach is, that
it allows to process test case files in parallel to Logstash, instead of
starting a new Logstash instance for every test case file. Because Logstash
is known to start slowly, this increases the time needed significantly,
especially if there are lots of different test case files.

For the test cases to work properly together with the unix domain socket input,
the test case files need to include the property `codec` set to the value `line`
(or `json_lines`, if json formatted input should be processed).


### The `--logstash-arg` flag

The `--logstash-arg` flag is used to supply additional command line
arguments or flags for Logstash. Those arguments are not processed by
Logstash Filter Verifier other than just forwarding them to Logstash.
For flags consisting of a flag name and a value, for both a seperate
`--logstash-arg` in the correct order has to be provided.  Because
values, starting with one or two dashes (`-`) are treated as flag by
Logstash Filter Verifier, for those flags the value _must_ not be
separated using a space but they have to be separated from the flag
with the equal sign (`=`).

For example to set the Logstash node name the following arguments have
to be provided to Logstash Filter Verifier:

    --logstash-arg=--node.name --logstash-arg MyInstanceName


### Logstash compatibility

Different versions of Logstash behave slightly differently and changes
in Logstash may require changes in Logstash Filter Verifier. Upon
startup, the program will attempt to auto-detect the version of
Logstash used and will use this information to adapt its own behavior.

Starting with Logstash 5.0 finding out the Logstash version is very
quick but in previous versions the version string was printed by Ruby
code in the JVM so it took several seconds. To avoid this you can use
the `--logstash-version` flag to tell Logstash Filter Verifier which
version of Logstash it should expect. Example:

    logstash-filter-verifier ... --logstash-version 2.4.0


### Windows compatibility

Logstash Filter Verifier has been reported to work on Windows, but
this isn't tested by the author and it's not guaranteed to work. There
are a couple of known quirks that are easy to work around:

* It won't guess the location of your Logstash executable so you'll have
  to manually provide it with the `--logstash-path` flag.
* The default value of the `--diff-command` is `diff -u` which won't work
  on typical Windows machines. You'll have to explicitly select which diff
  tool to use.


## Known limitations and future work

* Some log formats don't include all timestamp components. For
  example, most syslog formats don't include the year. This should be
  dealt with somehow.


## License

This software is copyright 2015–2020 by Magnus Bäck <<magnus@noun.se>>
and licensed under the Apache 2.0 license. See the LICENSE file for the full
license text.
