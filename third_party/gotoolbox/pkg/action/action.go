package action

// Task represents a function that performs an action and returns an error if it fails.
type Action func() error

// Executor holds the global state and any error encountered during task execution.
type Executor struct {
	err error
}

// NewExecutor initializes a new GlobalExecutor instance.
func NewExecutor() *Executor {
	return &Executor{}
}

// Execute runs a series of tasks in order, stopping at the first error.
func (ge *Executor) Execute(actions []Action) error {
	for _, task := range actions {
		ge.err = task()
		if ge.err != nil {
			return ge.err
		}
	}
	return nil
}

func Execute(err error, task func() error) error {
	if err != nil {
		return err
	}
	err = task()
	return err
}
