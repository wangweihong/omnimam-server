package example

import (
	"net/http"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/gotoolbox/pkg/sets"
)

type ErrCode struct {
	// Code refers to the code of the ErrCode.
	code int

	// HTTP status that should be used for the associated error code.
	http int

	// External (user) facing error text.
	message map[string]string
}

var _ errors.Coder = &ErrCode{}

// Code returns the integer code of ErrCode.
func (coder ErrCode) Code() int {
	return coder.code
}

// Reference returns the reference document.
func (coder ErrCode) Message() map[string]string {
	return coder.message
}

// Reference returns the reference document.
func (coder ErrCode) String() string {
	if coder.message != nil {
		msg := coder.message[errors.MessageLangENKey]
		return msg
	}
	return ""
}

// HTTPStatus returns the associated HTTP status code, if any. Otherwise,
// returns 200.
func (coder ErrCode) HTTPStatus() int {
	if coder.http == 0 {
		return http.StatusInternalServerError
	}

	return coder.http
}

func register(code int, httpStatus int, message map[string]string) {
	if !sets.NewInt(200, 400, 401, 403, 404, 500).Has(httpStatus) {
		panic("http code not in `200, 400, 401, 403, 404, 500`")
	}

	coder := &ErrCode{
		code:    code,
		http:    httpStatus,
		message: message,
	}

	errors.MustRegister(coder)
}
