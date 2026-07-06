package maputil_test

import (
	"encoding/json"
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/maputil"
)

var data map[string]any

func initDataSet(t *testing.T) {
	bd, err := os.ReadFile("./testdata/file.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(bd, &data); err != nil {
		t.Fatal(err)
	}
}

func TestGetFromMapInterface(t *testing.T) {
	initDataSet(t)
	Convey("TestGet*FromMapInterface", t, func() {
		So(maputil.GetStringFromMapInterface(data, "key"), ShouldEqual, "PRODUCT1-2130")
		So(maputil.GetStringFromMapInterface(data, "noexist"), ShouldEqual, "")
		So(maputil.GetStringFromMapInterface(data, "id"), ShouldEqual, "")

		So(maputil.GetFloat64FromMapInterface(data, "id"), ShouldEqual, 1233)

		fields := maputil.GetMapFromMapInterface(data, "fields")
		So(fields, ShouldNotBeNil)
		creator := maputil.GetMapFromMapInterface(fields, "creator")
		So(maputil.GetStringFromMapInterface(creator, "name"), ShouldEqual, "用户2")

		So(maputil.GetStringFromMapInterface(data, "name", "fields", "creator"), ShouldEqual, "用户2")
		So(maputil.GetStringFromMapInterface(data, "key"), ShouldEqual, "PRODUCT1-2130")
		So(maputil.GetStringFromMapInterface(data, ""), ShouldEqual, "")

		//not exist or not map
		So(maputil.GetStringFromMapInterface(data, "name", "fields", "notexist"), ShouldEqual, "")
		So(maputil.GetStringFromMapInterface(data, "name", "fields", "components"), ShouldEqual, "")

		//slice
		So(maputil.GetMapSliceFromMapInterface(data, "components", "fields"), ShouldNotBeNil)
		So(maputil.GetMapSliceFromMapInterface(data, "id"), ShouldBeNil)

		//map[string]interface
		So(maputil.GetMapFromMapInterface(data, "id"), ShouldBeNil)
		So(maputil.GetMapFromMapInterface(data, "components", "fields"), ShouldBeNil)
		So(maputil.GetMapFromMapInterface(data, "creator", "fields"), ShouldNotBeNil)

		// string slice
		So(maputil.GetStringSliceFromMapInterface(data, "slicestring"), ShouldResemble, []string{"s1", "s2"})
		So(maputil.GetStringSliceFromMapInterface(data, "intstring"), ShouldBeNil)
		So(maputil.GetStringSliceFromMapInterface(data, "sprint", "fields"), ShouldResemble, []string{"202408", "2024011"})
		So(maputil.GetStringSliceFromMapInterface(data, "name", "fields", "components"), ShouldResemble, []string{"controller", "storage"})

	})
}
