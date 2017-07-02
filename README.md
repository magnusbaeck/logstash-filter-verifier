# Logstash Filter Verifier

[![Travis](https://travis-ci.org/magnusbaeck/logstash-filter-verifier.svg?branch=master)](https://travis-ci.org/magnusbaeck/logstash-filter-verifier)
[![GoReportCard](http://goreportcard.com/badge/magnusbaeck/logstash-filter-verifier)](http://goreportcard.com/report/magnusbaeck/logstash-filter-verifier)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://raw.githubusercontent.com/magnusbaeck/logstash-filter-verifier/master/LICENSE)

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

This is where Logstash Filter Verifier comes in. In lets you define
test case files containing lines of input together with the expected
output from Logstash. Pass one of more such test case files to
Logstash Filter Verifier together with all of your Logstash filter
configuration files and it'll run Logstash for you and verify that
Logstash actually return what you expect.

Before you can run Logstash Filter Verifier you need to compile
it. After covering that, let's start with a simple example and follow
up with reference documentation.

## Installing

All releases of Logstash Filter Verifier are published in binary form
for the most common platforms at
[github.com/magnusbaeck/logstash-filter-verifier/releases](https://github.com/magnusbaeck/logstash-filter-verifier/releases).

If you need to run the program on other platforms or if you want to
modify the program yourself you can build and use it on any platform
for which a [Go](https://golang.org/) compiler is available. Pretty
much any platform where Logstash runs should be fine, including
Windows.

Many Linux distributions make some version of the Go compiler easily
installable, but otherwise you can [download and install the latest
version](https://golang.org/dl/). You should be able to compile the
source code with any reasonable up to date version of the Go compiler.
The build process also requires GNU make and other GNU tools. When
building on a Windows system you need a basic [cygwin](http://cygwin.com/)
installation.

To download and compile the source, run these commands (pick another
directory name if you like):

    $ mkdir ~/go
    $ cd ~/go
    $ export GOPATH=$(pwd)
    $ go get -d github.com/magnusbaeck/logstash-filter-verifier
    $ cd src/github.com/magnusbaeck/logstash-filter-verifier
    $ make

If successful you'll find an executable in the current directory. The
two last commands can be replaced with an invocation of the makefile
with `make`.

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

Test case files are in JSON format and contain a single object with
about a handful of supported properties.

```javascript
{
  "fields": {
    "type": "syslog"
  },
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
```

In this example, `type` is set to "syslog" which means that the input
events in this test case will have that in their `type` field when
they're passed to Logstash. Next, in `input`, we define a single test
string that we want to feed through Logstash, and the `expected` array
contains a one-element object with the event we expect Logstash to
emit for the given input.

Note that UTC is the assumed timezone for input events to avoid
different behavior depending on the timezone of the machine where
Logstash Filter Verifier happens to run. This won't affect time
formats that include a timezone.

This command will run this test case file through
Logstash Filter Verifier (replace all "path/to" with the actual paths
to the files, obviously):

    $ path/to/logstash-filter-verifier path/to/syslog.json path/to/filters

If the test is successful, Logstash Filter Verifier will terminate
with a zero exit code and (almost) no output. If the test fails it'll
run `diff -u` to compare the pretty-printed JSON representation of the
expected and actual events.

The actual event emitted by Logstash will contain a `@version` field,
but since that field isn't interesting it's ignored by default when
reading the actual event. Hence we don't need to include it in the
expected event either. Additional fields can be ignored with the
`ignore` array property in the test case file (see details below).

### JSON messages

I always prefer to configure application to emit JSON objects
whenever possible so that I don't have to write complex and/or
ambiguous grok expressions. Here's an example:

```javascript
{"message": "This is a test message", "client": "127.0.0.1", "host": "myhost", "time": "2015-10-06T20:55:29Z"}
```

When you feed events like this to Logstash it's likely that the
input used will have its codec set to "json_lines". This is something we
should mimic on the Logstash Filter Verifier side too. Use `codec` for
that:

```javascript
{
  "fields": {
    "type": "app"
  }
  "codec": "json_lines",
  "ignore": ["host"],
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
```

There are a few points to be made here:

* The double quotes inside the string must be escaped.
* The filters being tested here use Logstash's [dns
  filter](https://www.elastic.co/guide/en/logstash/current/plugins-filters-dns.html)
  to transform the IP address in the "client" field into a hostname
  and copy the original IP address into the "clientip" field. To avoid
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
* `expected`: An array of JSON objects with the events to be
  expected. They will be compared to the actual events produced by the
  Logstash process.
* `fields`: An object containing the fields that all input messages
  should have. This is vital since filters typically are configured
  based on the event's type and/or tags. Scalar values (strings,
  numbers, and booleans) are supported, as are objects (containing
  scalars, arrays and nested objects), arrays of scalars and nested arrays.
  The only combination which is not allowed are objects within arrays.
* `ignore`: An array with the names of the fields that should be
  removed from the events that Logstash emit. This is for example
  useful for dynamically generated fields whose contents can't be
  predicted and hardwired into the test case file.
* `input`: An array with the lines of input (each line being a string)
  that should be fed to the Logstash process.
* `testcases`: An array of test case hashes, consisting of a field `input`
  and a field `expected`, which work the same as the above mentioned
  `input` and `expected`, but allow to have the input and the expected
  event close together in the test case file, which offers a better
  overview. An optional `description` field can be used to describe the
  test case, e.g. as documentation. The description will be included in
  the program's progress messages.

## Migrate to testcases

To migrate test case files from the old to the new config format, which uses
the `testcases` array to keep the fields `input` and `expected` next
to each other, the following command using [jq](https://stedolan.github.io/jq/)
could be used (run this command in the directory containing the test case
files):

```
for f in `ls -1 *.json`; do jq '{ codec, fields, ignore, testcases:[[.input[]], [.expected[]]] | transpose | map({input: [.[0]], expected: [.[1]]})}' $f > $f.migrated && mv $f.migrated $f; done
```

This command only works for test case files, where for every line in
`input` an element in `expected` exists.

## Notes about the flag --sockets

The command line flag `--sockets` allows to use unix domain sockets instead of
stdin to send the input to Logstash. The advantage of this approach is, that
it allows to process test case files in parallel to Logstash, instead of
starting a new Logstash instance for every test case file. Because Logstash
is known to start slowly, this increases the time needed significantly,
especially if there are lots of different test case files.

For the test cases to work properly together with the unix domain socket input,
the test case files need to include the property `codec` set to the value `line`
(or `json_lines`, if json formatted input should be processed).

## Notes about flag --logstash-arg

The `--logstash-arg` flag is used to supply additional command line arguments
or flags for Logstash. Those arguments are not processed by Logstash Filter
Verifier other than just forwarding them to Logstash.
For flags consisting of a flag name and a value, for both a seperate
`--logstash-arg` in the correct order has to be provided.
Because values, starting with one or two dashes (`-`) are treated as flag
by Logstash Filter Verifier, for those flags the value MUST not be separated
using a space but they have to be separated from the flag with the equal sign (`=`).

For example to set the Logstash node name the following arguments have to be provided
to Logstash Filter Verifier:

```
--logstash-arg=--node.name --logstash-arg MyInstanceName
```

## Notes about Logstash compatibility

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

## Known limitations and future work

* Some log formats don't include all timestamp components. For
  example, most syslog formats don't include the year. This should be
  dealt with somehow.
* JSON files are tedious to write for a human with brackets, braces,
  double quotes, and escaped double quotes everywhere and no native
  support for comments. We should support YAML in addition to JSON to
  make it more pleasant to write test case files.
* All Logstash processes are run serially. By running them in parallel
  the execution time can be reduced drastically on multi-core
  machines.
* There have been reports regarding problems in combination with X-Pack plugins
  for Logstash ([Issue #31](https://github.com/magnusbaeck/logstash-filter-verifier/issues/31)).
  One possibility to resolve this problem is to disable the monitoring
  with the following configuration option in `logstash.yml`: `xpack.monitoring.enabled: false`.
* Support for Logstash 5.0 is incomplete but will work under most
  circumstances. The `--logstash-arg` flag (described above) may
  come in handy. See
  [Issue #8](https://github.com/magnusbaeck/logstash-filter-verifier/issues/8)
  for the current status.

## License

This software is copyright 2015-2016 by Magnus Bäck <<magnus@noun.se>>
and licensed under the Apache 2.0 license. See the LICENSE file for the full
license text.
