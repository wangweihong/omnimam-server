package compareutil_test

import (
	"reflect"
	"sort"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/wangweihong/gotoolbox/pkg/compareutil"
	"github.com/wangweihong/gotoolbox/pkg/randutil"
)

type TestStruct struct {
	ID          int
	ProductType string
	ProductNo   string
}

func TestStructReflectValueCompare(t *testing.T) {
	Convey("compare", t, func() {
		t1 := TestStruct{
			ID:          1,
			ProductType: "abc",
		}
		t2 := TestStruct{
			ID:          2,
			ProductType: "efg",
		}
		t3 := &TestStruct{
			ID:          3,
			ProductType: "efg",
		}
		v1 := reflect.ValueOf(t1)
		v2 := reflect.ValueOf(t2)
		v3 := reflect.ValueOf(t3)

		So(compareutil.StructReflectValueCompare(v1, v2, "ID"), ShouldEqual, -1)
		So(compareutil.StructReflectValueCompare(v1, v3, "ID"), ShouldEqual, 0)
	})
}

func BenchmarkStructReflectValueCompare(b *testing.B) {
	t1 := TestStruct{
		ID:          1,
		ProductType: "abc",
	}
	t2 := TestStruct{
		ID:          2,
		ProductType: "efg",
	}

	v1 := reflect.ValueOf(t1)
	v2 := reflect.ValueOf(t2)

	for i := 0; i < b.N; i++ {
		compareutil.StructReflectValueCompare(v1, v2, "ID")
	}
}

func generateRandomData(size int) []TestStruct {
	data := make([]TestStruct, size)
	for i := range data {
		data[i] = TestStruct{
			ID:          randutil.RandNumRange(0, 1000),
			ProductType: randutil.RandNumSets(4),
			ProductNo:   randutil.RandNumSets(4),
		}
	}
	return data
}

// 约132秒
func BenchmarkMillion(b *testing.B) {
	data := generateRandomData(10000000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sort.SliceStable(data, func(i, j int) bool {
			f1 := reflect.ValueOf(data[i])
			f2 := reflect.ValueOf(data[j])

			r := compareutil.StructReflectValueCompare(f1, f2, "ProductNo")
			switch r {
			case 1:
				return true
			case -1:
				return false
			}
			return data[i].ID < data[j].ID
		})
	}
}

// 约20秒
func BenchmarkMillion2(b *testing.B) {
	data := generateRandomData(10000000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sort.SliceStable(data, func(i, j int) bool {
			return data[i].ID < data[j].ID
		})
	}
}
