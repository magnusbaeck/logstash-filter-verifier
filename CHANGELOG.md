Logstash Filter Verifier Change Log
===================================

All timestamps are in the Europe/Stockholm timezone.


1.6.0 (2020-01-02)
------------------

  * Upgraded the Go compiler to 1.13 and transitioned to using Go
    modules for dependency management.
  * Dropped Debian packaging support.
  * Allow test case file to be in YAML format instead of JSON.
  * Support Logstash's field reference syntax (`[field][subfield]`)
    in the `ignore` test case file option to ignore only certain
    subfields.


1.5.1 (2019-07-11)
------------------

  * The temporary directory to which the configuration files are
    copied is now created with mode 0700. This addresses a security
    vulnerability when configuration files contain secrets.
  * Test flakiness when used with Logstash 7 is addressed by limiting
    the pipeline batch size to a single message.


1.5.0 (2018-09-09)
------------------

  * Allow keeping multiple environment variables with --keep-env.
  * Input and outputs sections are automatically removed from the
    Logstash configurations under test. That way you don't have to
    segregate different kinds of plugin into different configuration
    files and take care to never pass any files containing inputs
    and outputs to LFV.
  * The command line argument for Logstash configuration files to
    test can now include directories and not just files.
  * Testcase files with the "fields" key set to null no longer causes
    LFV to panic.
  * Fix "make install" on Mac OS X by omitting the -s/--strip option
    to install(1).


1.4.1 (2018-01-01)
------------------

  * Fix for a crash when using --sockets with a testcase file without
    a "fields" option.


1.4.0 (2017-12-17)
------------------

  * Full support for Logstash 5 and later. By default the version used
    with LFV is auto-detected (in order to adapt the LFV behavior) but
    this can be overridden with --logstash-version.
  * --logstash-path no longer sets the singular Logstash path but rather
    adds an extra entry to the list of locations that are checked for a
    Logstash installation.
  * When using --sockets a magic [@metadata][__lfv_testcase] field was
    added, but it would clobber any existing @metadata fields. This field
    is now appended to any existing @metadata fields.


1.3.0 (2017-05-21)
------------------

  * The --sockets option is incompatible with a couple of input codecs
    that happen to work if you don't use that option. Warn the use about
    this.
  * Addition of a --logstash-arg option that allows the user to pass
    additional arguments to all started Logstash processes.
  * The PATH environment variable is by default passed on to started
    Logstash processes. This fixes a bug where Logstash under some
    circumstances isn't able to find the JVM.


1.2.1 (2017-04-05)
------------------

  * Addition of --sockets-timeout option to control how long to
    wait for Logstash to start up and become ready to process
    events when using the --sockets option.
  * Status and progress messages are now written to stdout instead
    of stderr.
  * Addition of a "description" field for test cases that e.g. can be
    used as a short piece of documentation.


1.2.0 (2017-02-21)
------------------

  * Logstash 5.2 compatibility. Issues still exist with Logstash 5.0
    and possibly 5.1.
  * Addition of --sockets option that causes the program to use Unix
    domain sockets to pass inputs to Logstash, enabling a single
    Logstash process to be used for multiple test case files which
    has the potential to dramatically shorten the execution time.
  * Go 1.8 is now required for compiling.
  * Support for a new test case file format where pairs of input and
    expected output lines are store together. For now both formats
    work.
  * Addition of --logstash-output option that causes the Logstash
    output to be emitted.
  * JSON parse errors for test case files are reported with line and
    column details to make it easier to find the problem.
  * The makefile now supports a GOPATH variable with multiple paths.
  * When adding fields to input events with the `fields` option, nested
    fields may now be objects.
  * Large floating point numbers are now formatted in a way that's
    acceptable to Logstash.
  * Minor improvements in the messages given when running the program.


1.1.1 (2016-07-31)
------------------

  * Multiple filter configuration files now work. Previously only one
    of the files would be picked up by Logstash, possibly resulting in
    incorrect test results.
  * When invoking with --help to get command-line help, the exit code
    is now zero.


1.1.0 (2016-02-25)
------------------

  * Adds the --keep-env option to keep select environment variables
    when invoking Logstash. Useful to propagate JAVA_HOME and any
    other variables needed by Logstash.
  * If the Logstash child process terminates with a non-zero exit code,
    show the stdout/stderr output from the command rather than solely
    relying on the log output. If Logstash fails very early, e.g. before
    the JVM starts up, there won't be a logfile.


1.0.1 (2016-02-11)
------------------

  * Fixes Logstash 2.2.0 incompatibility problem.
  * If the Logstash child process terminates with a non-zero exit code,
    the contents of Logstash's log file is now included in the error
    message.


1.0.0 (2016-01-10)
------------------

  * Initial release.
