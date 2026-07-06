package code

// example-server: example-server related code.
//
//go:generate codegen -type=int

const (
	ErrUserNotFound int = iota + 110001
)
