//nolint:errorlint
package errors

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/json"
)

type ServiceInfo struct {
	Host string
	Pid  int
	Name string
}

type ServiceStack struct {
	Service ServiceInfo
	Stacks  []string
}

type Status struct {
	HTTPStatus int
	Code       int
	Message    map[string]string
	Desc       string
	Cause      []ServiceStack
}

func (s *Status) Error() *status {
	return &status{
		stack: callers(),
		code:  s.Code,
		cause: s.Cause,
		err:   errors.New(s.Desc),
	}
}

// UpdateStatus 添加当前服务的调用栈
func (s *Status) UpdateStatus() *Status {
	e := s.Error()
	return e.ToStatus()
}

var _ error = &status{}

// status 主要用于网络服务间的错误传递
// 调用链: 服务A --> 服务B --> 服务C
// 场景:
// 当服务C某处发生错误时，通过status设置错误码，记录错误调用栈，然后转换成Status传递给服务B
// 服务B接收到C的返回. 解析后得到服务C的错误码/错误栈. 服务B可以选择使用服务C的错误码/创建新的错误码,创建B当前调用错误栈, 继承服务C的错误栈.
// 服务A同上。
// 通过以上方式，从A的回应中就能定位到整个调用链中错误发生的具体现场。
// 大型系统应使用OpenTelemetry.
type status struct {
	*stack
	code int
	// 服务状态栈
	cause []ServiceStack
	err   error
}

// NewDesc generate a new status error with `code`,`desc` and current project stack
func NewStatus(code int, msg string) error {
	errStack := &status{
		stack: callers(),
		err:   errors.New(msg),
		code:  code,
	}

	return errStack
}

func NewStatusF(code int, format string, args ...any) error {
	status := &status{
		stack: callers(),
		err:   fmt.Errorf(format, args...),
		code:  code,
	}

	return status
}

// 基于某个错误，设置指定错误码
func WrapStatus(err error, code int) error {
	if err == nil {
		return nil
	}
	st := FromError(err)
	st.code = code

	return st
}

func ToStatus(err error) *Status {
	if err == nil {
		return &Status{
			HTTPStatus: success.httpCode,
			Code:       success.code,
			Message:    success.message,
		}
	}

	st := &status{
		err:  err,
		code: unknown.code,
	}
	switch e := err.(type) {
	case *status:

		cur := callersDepth(-1, 4)
		newStack := MergeStack(cur, e.stack)

		st.stack = newStack
		st.code = e.code
		st.cause = e.cause

	// error is generate from github.com/pkg/errors
	case StdStackTracer:
		st.stack = toStackTrace(e.StackTrace()).Stack()

	// error is generate from New/WithStack/WithCode/WithMessage
	case StackTracer:
		st.stack = e.StackTrace().Stack()
	default:
		st.stack = callersDepth(-1, 4)
	}
	return st.ToStatus()
}

func (m *status) ToStatus() *Status {
	coder, ok := codes[m.code]
	if !ok {
		coder = unknown
	}
	cause := m.cause

	si := GetModuleInfo()
	newCause := make([]ServiceStack, 0, len(cause)+1)
	newCause = append(newCause, ServiceStack{
		Service: si,
		Stacks:  processProjectStacks(m.StackTrace()),
	})
	newCause = append(newCause, cause...)
	return &Status{
		Code:       coder.Code(),
		HTTPStatus: coder.HTTPStatus(),
		Message:    coder.Message(),
		Desc:       m.err.Error(),
		Cause:      newCause,
	}
}

func (m *status) Error() string {
	coder, ok := codes[m.code]
	if !ok {
		coder = unknown
	}

	return fmt.Sprintf("%v:%v", coder.String(), Cause(m).Error())
}

// Error return the externally-safe error message.
//func (w *status) Error() string { return fmt.Sprintf("%v", w) }

// Cause return the cause of the withCode error.
func (m *status) Cause() error { return m.err }

// Unwrap provides compatibility for Go 1.13 error chains.
func (m *status) Unwrap() error { return m.err }

func (m *status) ToBasicJson() map[string]any {
	out := make(map[string]any)
	//out["desc"] = m.description
	//out["message"] = m.Message()
	//out["code"] = m.Code()
	out["stack"] = m.stack.StackTrace()

	return out
}
func (m *status) ToDetailJson() map[string]any {
	out := make(map[string]any)
	//out["desc"] = m.description
	//out["stack"] = m.StackInfo()
	//out["message"] = m.Message()
	//out["code"] = m.Code()
	//out["http"] = m.HTTPStatus()
	out["stack"] = m.stack.StackTrace()
	return out
}

// FromError parse any error into *status.
// nil error will return nil directly, caller should handle nil *status.
// None status error will be parsed as ErrUnknown.
// NOTE: `*status is nil` doesn't equal to `error is nil`.
func FromError(err error) *status {
	if err == nil {
		return nil
	}
	st := &status{
		err:  err,
		code: unknown.code,
	}
	//fmt.Println(reflect.TypeOf(err))
	switch e := err.(type) {
	case *status:
		newStack := MergeStack(callersDepth(-1, 4), e.stack)
		st.stack = newStack
		st.code = e.code
	// error is generate from github.com/pkg/errors
	case StdStackTracer:
		est := toStackTrace(e.StackTrace())
		st.stack = est.Stack()

	// error is generate from New/Errorf/WithStack/WithCode/WithMessage
	case StackTracer:
		frames := e.StackTrace()
		//fmt.Println("frames:", len(frames))
		st.stack = frames.Stack()
	default:
		st.stack = callers()
	}
	return st
}

//
//func newStack(code int, caller string) string {
//	return ModuleString() + fmt.Sprintf(",code:%d,", code) + caller
//}

/*
Format implements fmt.Formatter. https://golang.org/pkg/fmt/#hdr-Printing

Verbs:
    %s  - Returns the user-safe error string mapped to the error code or
      ┊   the error message if none is specified.
    %v      Alias for %s

Flags:
     #      JSON formatted output, useful for logging
     +      Output full error stack details, useful for debugging

Examples:
	%s:    OpenFileError:file not exist
	%q:    "OpenFileError:file not exist"
	%v:    OpenFileError:file not exist
	%+v:   OpenFileError:file not exist [host:127.0.0.1,pid:8536,module:testing,code:101,file:error_test.go,func:1,line:55]
	%#v:   {"code":101,"desc":"file not exist","message":{"cn":"访问文件失败","en":"OpenFileError"}}
	%#+v:  {"code":101,"desc":"file not exist","http":200,"message":{"cn":"访问文件失败","en":"OpenFileError"},"stack":[{"host":"127.0.0.1","pid":"6716","module":"testing","code":"101","file_name":"error_test.go","func_name":"1","line":"55"}]}
*/

func (m *status) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		str := bytes.NewBuffer([]byte{})

		var (
			flagDetail bool
			modeJSON   bool
		)

		if s.Flag('#') {
			modeJSON = true
		}

		if s.Flag('+') {
			flagDetail = true
		}

		if modeJSON {
			byteData, _ := json.Marshal(m.formatJson(flagDetail))
			str.Write(byteData)
		} else {
			if flagDetail {
				fmt.Fprintf(str, "%s %v", m.Error(), m.stack)
			} else {
				fmt.Fprintf(str, "%s", m.Error())
			}
		}
		fmt.Fprintf(s, "%s", strings.Trim(str.String(), "\r\n\t"))
	case 's':
		_, _ = io.WriteString(s, m.Error())
	case 'q':
		fmt.Fprintf(s, "%q", m.Error())
	}
}

func (m *status) formatJson(detail bool) map[string]any {
	data := m.ToBasicJson()
	if detail {
		data = m.ToDetailJson()
	}
	return data
}
