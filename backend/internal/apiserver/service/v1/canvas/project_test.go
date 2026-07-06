package canvas

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMergeGraphValue(t *testing.T) {
	Convey("merge graph values", t, func() {
		Convey("appends array values", func() {
			got := mergeGraphValue([]any{"a"}, []any{"b"})
			So(got, ShouldResemble, []any{"a", "b"})
		})

		Convey("overlays map values", func() {
			got := mergeGraphValue(map[string]any{"a": 1}, map[string]any{"b": 2})
			So(got, ShouldResemble, map[string]any{"a": 1, "b": 2})
		})

		Convey("keeps current value when incoming is empty", func() {
			got := mergeGraphValue([]any{"a"}, nil)
			So(got, ShouldResemble, []any{"a"})
		})
	})
}
