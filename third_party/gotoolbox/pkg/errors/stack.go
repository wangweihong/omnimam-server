package errors

import (
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/stringutil"

	stderrors "github.com/pkg/errors"
)

// Frame represents a program counter inside a stack frame.
// For historical reasons if Frame is interpreted as a uintptr
// its value represents the program counter + 1.
type Frame uintptr

// pc returns the program counter for this frame;
// multiple frames may have the same PC value.
func (f Frame) pc() uintptr { return uintptr(f) - 1 }

// file returns the full path to the file that contains the
// function for this Frame's pc.
func (f Frame) file() string {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return "unknown"
	}
	file, _ := fn.FileLine(f.pc())
	return file
}

func (f Frame) File() string {
	return f.file()
}

// line returns the line number of source code of the
// function for this Frame's pc.
func (f Frame) line() int {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return 0
	}
	_, line := fn.FileLine(f.pc())
	return line
}

func (f Frame) Line() int {
	return f.line()
}

// name returns the name of this function, if known.
func (f Frame) name() string {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return "unknown"
	}
	return fn.Name()
}

func (f Frame) Name() string {
	return f.name()
}

func (f Frame) string() string {
	name := f.name()
	if name == "unknown" {
		return name
	}
	return fmt.Sprintf("%s:%d %s", f.file(), f.line(), name)
}

func (f Frame) String() string {
	return f.string()
}

// Format formats the frame according to the fmt.Formatter interface.
//
//	%s    source file
//	%d    source line
//	%n    function name
//	%v    equivalent to %s:%d
//
// Format accepts flags that alter the printing of some verbs, as follows:
//
//	%+s   function name and path of source file relative to the compile time
//	      GOPATH separated by \n\t (<funcname>\n\t<path>)
//	%+v   equivalent to %+s:%d
func (f Frame) Format(s fmt.State, verb rune) {
	switch verb {
	case 's':
		switch {
		case s.Flag('+'):
			io.WriteString(s, f.name())
			io.WriteString(s, "\n\t")
			io.WriteString(s, f.file())
		default:
			io.WriteString(s, path.Base(f.file()))
		}
	case 'd':
		io.WriteString(s, strconv.Itoa(f.line()))
	case 'n':
		io.WriteString(s, funcname(f.name()))
	case 'v':
		f.Format(s, 's')
		io.WriteString(s, ":")
		f.Format(s, 'd')
	}
}

// MarshalText formats a stacktrace Frame as a text string. The output is the
// same as that of fmt.Sprintf("%+v", f), but without newlines or tabs.
func (f Frame) MarshalText() ([]byte, error) {
	name := f.name()
	if name == "unknown" {
		return []byte(name), nil
	}
	return []byte(fmt.Sprintf("%s %s:%d", name, f.file(), f.line())), nil
}

// StackTrace is stack of Frames from innermost (newest) to outermost (oldest).
type StackTrace []Frame

// Format formats the stack of Frames according to the fmt.Formatter interface.
//
//	%s	lists source files for each Frame in the stack
//	%v	lists the source file and line number for each Frame in the stack
//
// Format accepts flags that alter the printing of some verbs, as follows:
//
//	%+v   Prints filename, function, and line number for each Frame in the stack.
func (st StackTrace) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		switch {
		case s.Flag('+'):
			for _, f := range st {
				io.WriteString(s, "\n")
				f.Format(s, verb)
			}
		case s.Flag('#'):
			fmt.Fprintf(s, "%#v", []Frame(st))
		default:
			st.formatSlice(s, verb)
		}
	case 's':
		st.formatSlice(s, verb)
	}
}

func (st StackTrace) Stack() *stack {
	uis := make([]uintptr, 0, len(st))
	for _, f := range st {
		uis = append(uis, uintptr(f))
	}
	sk := stack(uis)
	return &sk
}

// formatSlice will format this StackTrace into the given buffer as a slice of
// Frame, only valid when called with '%s' or '%v'.
func (st StackTrace) formatSlice(s fmt.State, verb rune) {
	io.WriteString(s, "[")
	for i, f := range st {
		if i > 0 {
			io.WriteString(s, " ")
		}
		f.Format(s, verb)
	}
	io.WriteString(s, "]")
}

// List return info of  a slice of frame
func (st StackTrace) List() []string {
	fl := make([]string, 0, len(st))
	for _, f := range st {
		fl = append(fl, f.string())
	}
	return fl
}

// stack represents a stack of program counters.
type stack []uintptr

func (s *stack) Format(st fmt.State, verb rune) {
	switch verb {
	case 'v':
		switch {
		case st.Flag('+'):
			for _, pc := range *s {
				f := Frame(pc)
				fmt.Fprintf(st, "\n%+v", f)
			}
		}
	}
}

func (s *stack) StackTrace() StackTrace {
	f := make([]Frame, len(*s))
	for i := 0; i < len(f); i++ {
		f[i] = Frame((*s)[i])
	}
	return f
}

func callers() *stack {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	var st stack = pcs[0:n]
	return &st
}

// depth: 设置返回的栈的深度
// skip: 忽略栈前N个帧信息
func callersDepth(depth int, skip int) *stack {
	if depth < 0 {
		depth = 32
	}

	if skip < 0 || skip > depth {
		skip = 3
	}

	pcs := make([]uintptr, depth)
	n := runtime.Callers(skip, pcs[:])
	var st stack = pcs[0:n]
	return &st
}

// depth: 设置返回的栈的深度
// skip: 忽略栈前N个帧信息
func CallersDepth(depth int, skip int) *stack {
	return callersDepth(depth, skip)
}

// funcname removes the path prefix component of a function's name reported by func.Name().
func funcname(name string) string {
	i := strings.LastIndex(name, "/")
	name = name[i+1:]
	i = strings.Index(name, ".")
	return name[i+1:]
}

type StackTracer interface {
	StackTrace() StackTrace
}

type StdStackTracer interface {
	StackTrace() stderrors.StackTrace
}

// convert github/pkg/errors to stack
func toStackTrace(sts stderrors.StackTrace) StackTrace {
	stack := make([]Frame, 0, len(sts))
	for _, f := range sts {
		stack = append(stack, Frame(f))
	}
	return stack
}

// ProjectStacks 打印当前项目中调用栈(文件路径:行号 函数名), 忽略系统库或第三方调用库的栈。
func ProjectStacks() []string {
	fs := callersDepth(-1, 4).StackTrace()
	return processProjectStacks(fs)
}

// 如果对应的包里面使用了errors(只要实现了DebugTrace)，我们就可以捕获到最原始的现场(包括第三方包的现场)
// 如果只是为了避免栈在网络间传输的开销？将完整的栈交给opentelemetry.
func processProjectStacks(fs StackTrace) []string {
	infos := make([]string, 0, len(fs))
	for _, f := range fs {
		if !allStackEnabled() {
			if !strings.Contains(f.name(), currentModule.Name()) {
				continue
			}

			//avoid local filepath leak
			file := stringutil.RemoveSubBefore(f.file(), currentModule.Name())
			info := fmt.Sprintf("%s:%d %s'", file, f.line(), f.name())
			infos = append(infos, info)
		} else {
			infos = append(infos, f.String())
		}
	}
	return infos
}

// convert github/pkg/errors
func processStdStacks(fs stderrors.StackTrace) []string {
	st := toStackTrace(fs)
	return processProjectStacks(st)
}

func MergeStack(a, b *stack) *stack {
	frames := MergeFrame(a.StackTrace(), b.StackTrace())
	return StackTrace(frames).Stack()
}

// MergeFrame 比较m和b,从末尾开始将m中和b不同的部分，叠加到b前
func MergeFrame(m, b []Frame) []Frame {
	lenM, lenB := len(m), len(b)
	var offset int

	for {
		iM, iB := lenM-offset-1, lenB-offset-1

		if iM < 0 || iB < 0 {
			break
		}

		if m[iM] != b[iB] {
			break
		}
		offset += 1
	}

	if offset == lenM || offset == lenB {
		if lenM > lenB {
			return append(m[:lenM-lenB], b...)
		}
		return b
	}

	result := append(m[0:lenM-offset], b...)
	return result
}

func allStackEnabled() bool {
	debugEnv := os.Getenv("ERROR_STACK_ALL")
	return debugEnv != "" && debugEnv != "0"
}
