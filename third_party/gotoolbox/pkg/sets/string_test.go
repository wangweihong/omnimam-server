package sets_test

import (
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/wangweihong/gotoolbox/pkg/sets"
)

func TestString_ToSetString(t *testing.T) {
	Convey("TestStringBoolMap_ToSetString", t, func() {
		d := sets.NewString()
		d.Insert("stringa", "sstringb")

		d2 := d.FindMatch(func(s string, s2 string) bool {
			if strings.Contains(s, s2) {
				return true
			} else {
				return false
			}
		}, "string")

		So(d2.Len(), ShouldEqual, d.Len())
		d3 := d.FindMatch(func(s string, s2 string) bool {
			if strings.HasPrefix(s, s2) {
				return true
			} else {
				return false
			}
		}, "string")
		So(d3.Len(), ShouldEqual, 1)
		So(d3.List(), ShouldResemble, []string{"stringa"})

	})
}

func TestSetString(t *testing.T) {
	Convey("TestString", t, func() {
		d := sets.NewString()
		d.Insert("key1", "key2", "myKey")

		So(d.IsPrefixOf("myKeyOld"), ShouldBeTrue)
		So(d.IsPrefixOf("xxxx"), ShouldBeFalse)
		So(d.HasPrefix("my"), ShouldBeTrue)
		So(d.IsPrefixOf("xxxx"), ShouldBeFalse)

		So(d.IsSuffixOf("newmyKey"), ShouldBeTrue)
		So(d.IsSuffixOf("xxxx"), ShouldBeFalse)
		So(d.HasSuffix("Key"), ShouldBeTrue)
		So(d.HasSuffix("xxxx"), ShouldBeFalse)
	})
}
