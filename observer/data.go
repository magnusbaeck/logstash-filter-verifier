package observer

// DataObserver describes data shared between producer and consumer.
type DataObserver struct {
	start      bool
	end        bool
	testResult *ComparisonResult
}

// ComparisonResult permit to follow the test execution
type ComparisonResult struct {
	Name       string
	Status     bool
	Explain    string
	Path       string
	EventIndex int
}

// IsStart permit to know if is the first data
func (h *DataObserver) IsStart() bool {
	return h.start
}

// IsEnd permit to know if is the last data
func (h *DataObserver) IsEnd() bool {
	return h.end
}

// TestResult get the test result
func (h *DataObserver) TestResult() *ComparisonResult {
	return h.testResult
}

// NewDataObserver create a new data for observer
func NewDataObserver(testResult *ComparisonResult, start bool, end bool) *DataObserver {
	return &DataObserver{
		start:      start,
		end:        end,
		testResult: testResult,
	}
}
