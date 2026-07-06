package executil

func NewCommander() *Commander {
	return &Commander{}
}

type Commander struct {
	Err error
}

func (c Commander) Execute(command string, args ...string) (string, error) {
	if c.Err != nil {
		return "", c.Err
	}
	return Execute(command, args)
}
