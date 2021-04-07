// +build pprof

package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
)

func init() {
	go func() {
		err := http.ListenAndServe("localhost:6060", nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to start http server for pprof: %v", err)
		}
	}()
}
