package paging_test

import (
	"sort"
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/paging"

	. "github.com/smartystreets/goconvey/convey"
)

func TestPaging(t *testing.T) {
	Convey("", t, func() {
		s, e := paging.Index(0, 0, 0)
		So(s, ShouldEqual, 0)
		So(e, ShouldEqual, 0)

		s, e = paging.Index(0, -1, 0)
		So(s, ShouldEqual, 0)
		So(e, ShouldEqual, 0)

		s, e = paging.Index(10, -1, 0)
		So(s, ShouldEqual, 0)
		So(e, ShouldEqual, 10)

		s, e = paging.Index(10, -1, 5)
		So(s, ShouldEqual, 0)
		So(e, ShouldEqual, 5)

		s, e = paging.Index(10, 0, 5)
		So(s, ShouldEqual, 0)
		So(e, ShouldEqual, 5)

		s, e = paging.Index(10, 1, 5)
		So(s, ShouldEqual, 5)
		So(e, ShouldEqual, 10)

		s, e = paging.Index(10, 2, 5)
		So(s, ShouldEqual, 10)
		So(e, ShouldEqual, 10)
	})
}

func TestCombinePaging(t *testing.T) {
	Convey("组合分页，组合两种不同类型进行分页", t, func() {
		// 目录结构
		type Directory struct {
			Name string
			// 其他目录相关字段
		}

		// 文件结构
		type File struct {
			Name string
			// 其他文件相关字段
		}

		// 结合目录和文件的结构
		type CombinedItem struct {
			Directory *Directory // 目录
			File      *File      // 文件
		}

		// 假设有两个对象数组，一个是目录，一个是文件
		directories := []Directory{
			{Name: "Dir1"},
			{Name: "Dir2"},
			// 其他目录
		}

		sort.SliceStable(directories, func(i, j int) bool {
			return directories[i].Name < directories[j].Name
		})

		files := []File{
			{Name: "File1"},
			{Name: "File2"},
			// 其他文件
		}
		sort.SliceStable(files, func(i, j int) bool {
			return files[i].Name < files[j].Name
		})

		// 合并目录和文件到结构体切片
		var combinedItems []CombinedItem

		// 将目录加入结构体切片
		for _, dir := range directories {
			combinedItems = append(combinedItems, CombinedItem{Directory: &dir})
		}

		// 将文件加入结构体切片
		for _, file := range files {
			combinedItems = append(combinedItems, CombinedItem{File: &file})
		}

		s, e := paging.Index(len(combinedItems), 0, 3)
		t1 := combinedItems[s:e]
		So(t1[0].Directory, ShouldNotBeNil)
		So(t1[1].Directory, ShouldNotBeNil)
		So(t1[2].File, ShouldNotBeNil)
	})
}
