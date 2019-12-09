package observer

import (
	"fmt"
	"sort"

	"github.com/imkira/go-observer"
)

type Summary struct {
	NBOk int
	NBKo int
}

var (
	results       map[string]Summary
	globalSummary Summary
)

func RunSummaryObserver(prop observer.Property) {

	stream := prop.Observe()

	for {
		data := stream.Value().(*DataObserver)

		// Init struct to store result test
		if data.IsStart() {
			results = make(map[string]Summary)
			globalSummary = Summary{
				NBOk: 0,
				NBKo: 0,
			}
		}

		// Store result test
		if data.TestResult() != nil {
			val := data.TestResult()

			// Compute summary to display at the end and siplay current test status
			summary, ok := results[val.Path]
			if !ok {
				summary = Summary{
					NBOk: 0,
					NBKo: 0,
				}
			}
			if val.Status {
				summary.NBOk++
				globalSummary.NBOk++
				fmt.Printf("\u2611 %s from %s\n", val.Name, val.Path)
			} else {
				summary.NBKo++
				globalSummary.NBKo++
				fmt.Printf("\u2610 %s from %s:\n%s\n", val.Name, val.Path, val.Explain)
			}
			results[val.Path] = summary
		}

		// Display result on stdout
		if data.IsEnd() {
			var status string
			if globalSummary.NBKo == 0 {
				status = "\u2611"
			} else {
				status = "\u2610"
			}

			fmt.Printf("\nSummary: %s All tests : %d/%d\n", status, globalSummary.NBOk, globalSummary.NBOk+globalSummary.NBKo)

			// Ordering by keys name
			keys := make([]string, len(results))
			i := 0
			for key := range results {
				keys[i] = key
				i++
			}
			sort.Strings(keys)
			for _, key := range keys {
				summary := results[key]
				if summary.NBKo == 0 {
					status = "\u2611"
				} else {
					status = "\u2610"
				}

				fmt.Printf("\t%s %s : %d/%d\n", status, key, summary.NBOk, summary.NBOk+summary.NBKo)
			}
		}

		select {
		// wait for changes
		case <-stream.Changes():
			// advance to next value
			stream.Next()
		}
	}
}
