package example

//go:generate codegen -type=int

// example: policy errors.
const (

	// @HTTP 404
	// @MessageCN 策略未找到
	// @MessageEN Policy not found.
	ErrPolicyNotFound int = iota + 110201
)

// example: user errors.
const (

	// @HTTP 404
	// @MessageCN 用户未找到
	// @MessageEN User not found.
	ErrUserNotFound int = iota + 110202
)
