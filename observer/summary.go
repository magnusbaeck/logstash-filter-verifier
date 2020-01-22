package observer

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/imkira/go-observer"
	"github.com/magnusbaeck/logstash-filter-verifier/logging"
)

// Summary describe the summary of global test case.
// It count the number of success test and opposite
// the number of failed test.
type Summary struct {
	NumberOk    int
	NumberNotOk int
}

var log = logging.MustGetLogger()

// RunSummaryObserver lauch consummer witch is in responsible to
// print summary at the end of all tests cases.
func RunSummaryObserver(prop observer.Property) {
	var (
		results       map[string]Summary
		globalSummary Summary
	)

	stream := prop.Observe()

	for {
		data := stream.Value()

		switch dataType := reflect.TypeOf(data); dataType {
		// Init struct to store result test
		case reflect.TypeOf((*TestExecutionStart)(nil)).Elem():
			results = make(map[string]Summary)
			globalSummary = Summary{
				NumberOk:    0,
				NumberNotOk: 0,
			}
		// Display result on stdout
		case reflect.TypeOf((*TestExecutionEnd)(nil)).Elem():
			fmt.Printf("\nSummary: %s All tests : %d/%d\n", getIconStatus(globalSummary.NumberNotOk), globalSummary.NumberOk, globalSummary.NumberOk+globalSummary.NumberNotOk)

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

				fmt.Printf("\t%s %s : %d/%d\n", getIconStatus(summary.NumberNotOk), key, summary.NumberOk, summary.NumberOk+summary.NumberNotOk)
			}
		// Store result test
		case reflect.TypeOf((*ComparisonResult)(nil)).Elem():
			val := data.(ComparisonResult)

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
		default:
			log.Debugf("Receive data that we doesn't say how to manage it %+v", data)
		}

		// wait change
		<-stream.Changes()
		// advance to next value
		stream.Next()
	}
}

func getIconStatus(numberNotOk int) string {
	if numberNotOk == 0 {
		return "\u2611"
	}

	return "\u2610"
}
