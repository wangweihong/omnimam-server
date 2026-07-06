//nolint:errorlint
package errors

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/wangweihong/gotoolbox/pkg/sets"
)

const (
	MessageLangCNKey = "CN"
	MessageLangENKey = "EN"
)

var unknown = defaultCoder{
	code: 1,
	message: map[string]string{
		MessageLangCNKey: "未知错误码",
		MessageLangENKey: "Unknown Error Code",
	},
	httpCode: http.StatusInternalServerError,
}

var success = defaultCoder{
	code: 0,
	message: map[string]string{
		MessageLangCNKey: "成功",
		MessageLangENKey: "Success",
	},
	httpCode: http.StatusOK,
}

// defaultCoder is an error that has a message and a stack, but no caller.
type defaultCoder struct {
	// 错误码
	code int
	// 信息
	message map[string]string
	// http状态码
	httpCode int
}

type Coder interface {
	// HTTP status that should be used for the associated error code.
	HTTPStatus() int

	// External (user) facing error text.
	String() string

	// All language message
	Message() map[string]string

	// Code returns the code of the coder
	Code() int
}

func (f defaultCoder) HTTPStatus() int {
	return f.httpCode
}

func (f defaultCoder) Message() map[string]string {
	return f.message
}

func (f defaultCoder) Code() int {
	return f.code
}

func (f defaultCoder) String() string {
	return f.MessageEN()
}

func (f defaultCoder) MessageCN() string {
	if f.message != nil {
		msg := f.message[MessageLangCNKey]
		return msg
	}
	return ""
}

func (f defaultCoder) MessageEN() string {
	if f.message != nil {
		msg := f.message[MessageLangENKey]
		return msg
	}
	return ""
}

// codes contains a map of error codes to metadata.
var (
	codes   = map[int]Coder{}
	codeMux = &sync.RWMutex{}
)

func registerPre(coder Coder) {
	if coder.Code() == unknown.code {
		panic("code `1` is reserved by `errors` as unknown error code")
	}

	if coder.Code() == success.code {
		panic("code `0` is reserved by `errors` as success code")
	}

	if coder.Message() == nil {
		panic(
			fmt.Sprintf(
				"coder `%v` has no message map with key `%v` and `%v`",
				coder.Code(),
				MessageLangCNKey,
				MessageLangENKey,
			),
		)
	}

	if v := coder.Message()[MessageLangENKey]; strings.TrimSpace(v) == "" {
		panic(fmt.Sprintf("coder `%v` has message map  key `%v` value is empty", coder.Code(), MessageLangENKey))
	}

	if v := coder.Message()[MessageLangCNKey]; strings.TrimSpace(v) == "" {
		panic(fmt.Sprintf("coder `%v` has message map  key `%v` value is empty", coder.Code(), MessageLangCNKey))
	}

	found := sets.NewInt(200, 400, 401, 403, 404, 500).Has(coder.HTTPStatus())
	if !found {
		panic("http code not in `200, 400, 401, 403, 404, 500`")
	}
}

// Register register a user define error code.
// It will override the exist code.
func Register(coder Coder) {
	registerPre(coder)

	codeMux.Lock()
	defer codeMux.Unlock()

	codes[coder.Code()] = coder
}

// MustRegister register a user define error code.
// It will panic when the same Code already exist.
func MustRegister(coder Coder) {
	registerPre(coder)

	codeMux.Lock()
	defer codeMux.Unlock()

	if _, ok := codes[coder.Code()]; ok {
		panic(fmt.Sprintf("code: %d already exist", coder.Code()))
	}

	codes[coder.Code()] = coder
}

// ParseCoder parse any error into *withCode.
// nil error will return nil direct.
// None WithStack error will be parsed as ErrUnknown.
func ParseCoder(err error) Coder {
	if err == nil {
		return nil
	}

	if v, ok := err.(*withCode); ok {
		if coder, ok := codes[v.code]; ok {
			return coder
		}
	}

	return unknown
}

func Unknown() Coder {
	return unknown
}

func Success() Coder {
	return success
}

func Code(err error) int {
	if err != nil {
		if v, ok := err.(*withCode); ok {
			return v.code
		}
		return unknown.code
	}
	return success.code
}

// errorlint:ignore
// IsCode reports whether err (no chain) has the given error code.
func IsCode(err error, code int) bool {
	if err != nil {
		if v, ok := err.(*withCode); ok {
			if v.code == code {
				return true
			}
		}
	}

	return false
}

func IsSuccess(err error) bool {
	return IsCode(err, 0)
}

func IsSuccessCode(code int) bool {
	return code == 0
}

// HasCode reports whether any error in err's chain contains the given error code.
func HasCode(err error, code int) bool {
	if err != nil {
		if v, ok := err.(*withCode); ok {
			if v.code == code {
				return true
			}

			// 	逐级向上去获取withCode错误栈中,直到栈中包含某个错误.
			if v.cause != nil {
				return IsCode(v.cause, code)
			}
		}
	}

	return false
}

func NewCoder(code int, httpCode int, message map[string]string) Coder {
	return defaultCoder{
		code:     code,
		message:  message,
		httpCode: httpCode,
	}
}

//nolint:gochecknoinits
func init() {
	codes[success.code] = success
	codes[unknown.code] = unknown
}
