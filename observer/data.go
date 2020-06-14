package observer

// TestExecutionStart is empty struct to inform consumer that test execution has begun.
type TestExecutionStart struct{}

// TestExecutionEnd is empty struct to inform consumer that test execution has finished.
type TestExecutionEnd struct{}

// ComparisonResult describes the result of the execution of a single test case.
type ComparisonResult struct {
	Name       string
	Status     bool
	Explain    string
	Path       string
	EventIndex int
}
