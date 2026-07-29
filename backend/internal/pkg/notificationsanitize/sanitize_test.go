package notificationsanitize

import (
	"strings"
	"testing"
	"unicode/utf8"

	. "github.com/smartystreets/goconvey/convey"
)

func TestText(t *testing.T) {
	Convey("通知摘要清理 Header、JSON 凭证、URL、私网地址和内部栈", t, func() {
		input := `处理失败 Authorization: Bearer abc token=secret {"Authorization":"Bearer json-token","api_key":"key-value"} https://host/path 10.0.0.8
panic: boom
worker.go:42`
		got := Text(input, 1000)

		So(utf8.ValidString(got), ShouldBeTrue)
		So(strings.ToLower(got), ShouldNotContainSubstring, "abc")
		So(strings.ToLower(got), ShouldNotContainSubstring, "secret")
		So(strings.ToLower(got), ShouldNotContainSubstring, "json-token")
		So(strings.ToLower(got), ShouldNotContainSubstring, "key-value")
		So(got, ShouldNotContainSubstring, "https://")
		So(got, ShouldNotContainSubstring, "10.0.0.8")
		So(got, ShouldNotContainSubstring, ".go:")
	})

	Convey("通知摘要按 Unicode 字符安全截断", t, func() {
		got := Text("中文通知摘要", 4)

		So(utf8.ValidString(got), ShouldBeTrue)
		So(utf8.RuneCountInString(got), ShouldEqual, 4)
		So(got, ShouldEqual, "中文通知")
	})
}
