package logstash

import "github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"

type tailLogger struct {
	log logging.Logger
}

func (t tailLogger) Fatal(v ...interface{})                 { t.log.Fatal(v...) }
func (t tailLogger) Fatalf(format string, v ...interface{}) { t.log.Fatalf(format, v...) }
func (t tailLogger) Fatalln(v ...interface{})               { t.log.Fatal(v...) }
func (t tailLogger) Panic(v ...interface{})                 { t.log.Error(v...) }
func (t tailLogger) Panicf(format string, v ...interface{}) { t.log.Errorf(format, v...); panic("") }
func (t tailLogger) Panicln(v ...interface{})               { t.log.Error(v...); panic("") }
func (t tailLogger) Print(v ...interface{})                 { t.log.Info(v...) }
func (t tailLogger) Printf(format string, v ...interface{}) { t.log.Infof(format, v...) }
func (t tailLogger) Println(v ...interface{})               { t.log.Info(v...) }
