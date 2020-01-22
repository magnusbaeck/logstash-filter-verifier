package observer

// TestExecutionStart is empty struct to inform consumer that test begin
type TestExecutionStart struct{}

// TestExecutionEnd is empty struct to inform consumer that test is finished
type TestExecutionEnd struct{}

// ComparisonResult permit to follow the test execution
type ComparisonResult struct {
	Name       string
	Status     bool
	Explain    string
	Path       string
	EventIndex int
}
