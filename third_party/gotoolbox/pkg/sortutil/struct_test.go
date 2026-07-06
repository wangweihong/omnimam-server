package sortutil_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/sortutil"
)

func TestFieldTagSorter_Sort(t *testing.T) {
	Convey("TestFieldTagSorter_Sort", t, func() {
		type Entry struct {
			Key  int    `json:"key"`
			Name string `json:"name"`
		}
		var entrys []Entry
		entrys = append(entrys, Entry{
			Key:  3,
			Name: "c",
		})
		entrys = append(entrys, Entry{
			Key:  4,
			Name: "b",
		})
		entrys = append(entrys, Entry{
			Key:  5,
			Name: "a",
		})
		entrys = append(entrys, Entry{
			Key:  6,
			Name: "a",
		})
		err := sortutil.FieldTagSort(entrys, "name", true, func(i, j int) bool {
			return entrys[i].Key < entrys[j].Key
		}, nil)
		So(err, ShouldBeNil)
		So(entrys, ShouldResemble, []Entry{{5, "a"}, {6, "a"}, {4, "b"}, {3, "c"}})
	})
}

func TestStructSliceSort(t *testing.T) {

	Convey("TestFieldTagSorter_Sort", t, func() {
		Convey("TestFieldTagSorter_Sort_normal", func() {
			type Entry struct {
				Key        int `json:"key"`
				Value      int
				EqualValue int
			}
			type List struct {
				Entry Entry  `json:"entry"`
				Name  string `json:"name"`
			}
			l1, l2 := List{}, List{}
			l1.Name = "l1"
			l1.Entry.Key = 2
			l1.Entry.Value = 7
			l1.Entry.EqualValue = 100

			l2.Name = "l2"
			l2.Entry.Key = 1
			l2.Entry.Value = 8
			l2.Entry.EqualValue = 100

			var list []List
			list = append(list, l1, l2)

			sortutil.StructSliceSort(list, "name", false)
			So(list[0].Name, ShouldEqual, "l2")

			sortutil.StructSliceSort(list, "entry.key", false)
			So(list[0].Name, ShouldEqual, "l1")

			sortutil.StructSliceSort(list, "entry.Value", false)
			So(list[0].Name, ShouldEqual, "l2")

			sortutil.StructSliceSort(list, "entry.EqualValue entry.Value", false)
			So(list[0].Name, ShouldEqual, "l2")
		})

		Convey("TestFieldTagSorter_Sort_map", func() {
			type Entry struct {
				Map map[string]string
			}
			type List struct {
				Entry Entry  `json:"entry"`
				Name  string `json:"name"`
			}
			l1, l2 := List{}, List{}
			l1.Name = "l1"
			l1.Entry.Map = make(map[string]string)
			l1.Entry.Map["value"] = "b"

			l2.Name = "l2"
			l2.Entry.Map = make(map[string]string)
			l2.Entry.Map["value"] = "a"

			var list []List
			list = append(list, l1, l2)

			sortutil.StructSliceSort(list, "entry.Map.value", false)
			So(list[0].Name, ShouldEqual, "l1")
			sortutil.StructSliceSort(list, "entry.Map.value.", true)
			So(list[0].Name, ShouldEqual, "l2")
		})
	})
}
