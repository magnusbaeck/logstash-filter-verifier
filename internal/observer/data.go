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

// Interface defines the methods of an observer.
type Interface interface {
	// Start fires up the observer in a new goroutine.
	Start() error

	// Finalize waits for the observer to receive the final property value, process it,
	// and shut itself down.
	Finalize() error
}
