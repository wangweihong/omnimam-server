package callerutil

import (
	"fmt"
	"io"
	"path"
	"runtime"
	"strconv"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/stringutil"
)

// Frame represents a program counter inside a stack frame.
// For historical reasons if Frame is interpreted as a uintptr
// its value represents the program counter + 1.
type Frame uintptr

// pc returns the program counter for this frame;
// multiple frames may have the same PC value.
func (f Frame) pc() uintptr { return uintptr(f) - 1 }

// File returns the full path to the file that contains the
// function for this Frame's pc.
func (f Frame) File() string {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return "unknown"
	}
	file, _ := fn.FileLine(f.pc())
	return file
}

// Line returns the line number of source code of the
// function for this Frame's pc.
func (f Frame) Line() int {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return 0
	}
	_, line := fn.FileLine(f.pc())
	return line
}

// Name returns the name of this function, if known.
func (f Frame) Name() string {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return "unknown"
	}
	return fn.Name()
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
			io.WriteString(s, f.Name())
			io.WriteString(s, "\n\t")
			io.WriteString(s, f.File())
		default:
			io.WriteString(s, path.Base(f.File()))
		}
	case 'd':
		io.WriteString(s, strconv.Itoa(f.Line()))
	case 'n':
		io.WriteString(s, funcname(f.Name()))
	case 'v':
		f.Format(s, 's')
		io.WriteString(s, ":")
		f.Format(s, 'd')
	}
}

// MarshalText formats a stacktrace Frame as a text string. The output is the
// same as that of fmt.Sprintf("%+v", f), but without newlines or tabs.
func (f Frame) MarshalText() ([]byte, error) {
	name := f.Name()
	if name == "unknown" {
		return []byte(name), nil
	}
	return []byte(fmt.Sprintf("%s %s:%d", name, f.File(), f.Line())), nil
}

func (f Frame) String() string {
	name := f.Name()
	if name == "unknown" {
		return name
	}
	return fmt.Sprintf("%s:%d %s", f.File(), f.Line(), name)
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
		fl = append(fl, f.String())
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

func (s *stack) List() []string {
	return s.StackTrace().List()
}

func Callers() *stack {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	var st stack = pcs[0:n]
	return &st
}

// depth: 设置返回的栈的深度
// skip: 忽略栈前N个帧信息
func CallersDepth(depth int, skip int) *stack {
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

// funcname removes the path prefix component of a function's name reported by func.Name().
func funcname(name string) string {
	i := strings.LastIndex(name, "/")
	name = name[i+1:]
	i = strings.Index(name, ".")
	return name[i+1:]
}

// Stacks 打印调用栈(文件路径:行号 函数名), 根据depth选择返回的栈深度,
func Stacks(depth int) []string {
	fs := CallersDepth(depth, 3).StackTrace()
	infos := make([]string, 0, len(fs))
	for i := 0; i < len(fs); i++ {
		// ["C:/goprogram/src/github.com/wangweihong/gotoolbox/pkg/callerutil/stack_test.go:12 github.com/wangweihong/gotoolbox/pkg/callerutil_test.first"]
		infos = append(infos, fmt.Sprintf("%s:%d %s", fs[i].File(), fs[i].Line(), fs[i].Name()))
	}
	return infos
}

// StacksSkip 打印调用栈(文件路径:行号 函数名), 根据depth选择返回的栈深度, 忽略Skip长度
func StacksSkip(depth int) []string {
	return CallersDepth(depth, 3).StackTrace().List()
}

// ProjectStacks 打印当前项目中调用栈(文件路径:行号 函数名), 忽略系统库或第三方调用库的栈
func ProjectStacks(module string) []string {
	fs := CallersDepth(32, 3).StackTrace()

	infos := make([]string, 0, len(fs))
	for _, f := range fs {
		if strings.Contains(f.Name(), module) {
			// hide local file path leaks
			file := stringutil.RemoveSubBefore(f.File(), module)
			// ["github.com/wangweihong/gotoolbox/pkg/callerutil/stack_test.go:12 github.com/wangweihong/gotoolbox/pkg/callerutil_test.first"]
			infos = append(infos, fmt.Sprintf("%s:%d %s", file, f.Line(), f.Name()))
		}

	}
	return infos
}
