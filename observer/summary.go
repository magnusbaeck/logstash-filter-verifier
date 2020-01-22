package observer

import (
	"fmt"
	"sort"

	"github.com/imkira/go-observer"
)

// Summary describe the summary of global test case.
// It count the number of success test and opposite
// the number of failed test.
type Summary struct {
	NumberOk    int
	NumberNotOk int
}

var (
	results       map[string]Summary
	globalSummary Summary
)

// RunSummaryObserver lauch consummer witch is in responsible to
// print summary at the end of all tests cases.
func RunSummaryObserver(prop observer.Property) {
	stream := prop.Observe()

	for {
		data := stream.Value().(*DataObserver)

		// Init struct to store result test
		if data.IsStart() {
			results = make(map[string]Summary)
			globalSummary = Summary{
				NumberOk:    0,
				NumberNotOk: 0,
			}
		}

		// Store result test
		if data.TestResult() != nil {
			val := data.TestResult()

			// Compute summary to display at the end and siplay current test status
			summary, ok := results[val.Path]
			if !ok {
				summary = Summary{
					NumberOk:    0,
					NumberNotOk: 0,
				}
			}
			if val.Status {
				summary.NumberOk++
				globalSummary.NumberOk++
				fmt.Printf("\u2611 %s from %s\n", val.Name, val.Path)
			} else {
				summary.NumberNotOk++
				globalSummary.NumberNotOk++
				fmt.Printf("\u2610 %s from %s:\n%s\n", val.Name, val.Path, val.Explain)
			}
			results[val.Path] = summary
		}

		// Display result on stdout
		if data.IsEnd() {
			var status string
			if globalSummary.NumberNotOk == 0 {
				status = "\u2611"
			} else {
				status = "\u2610"
			}

			fmt.Printf("\nSummary: %s All tests : %d/%d\n", status, globalSummary.NumberOk, globalSummary.NumberOk+globalSummary.NumberNotOk)

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
				if summary.NumberNotOk == 0 {
					status = "\u2611"
				} else {
					status = "\u2610"
				}

				fmt.Printf("\t%s %s : %d/%d\n", status, key, summary.NumberOk, summary.NumberOk+summary.NumberNotOk)
			}
		}

		// wait change
		<-stream.Changes()
		// advance to next value
		stream.Next()
	}
}
