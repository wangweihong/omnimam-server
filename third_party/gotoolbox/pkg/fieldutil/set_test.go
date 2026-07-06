package fieldutil

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSetWhenTagValueMatch(t *testing.T) {
	Convey("结构体Tag字段为结构体", t, func() {
		type WrongConfig struct {
			IP       string `json:"ip"`        // 节点ip地址
			NodeName string `json:"node_name"` // 节点名
		}

		type NodeConfig struct {
			IP       string `json:"ip"`        // 节点ip地址
			NodeName string `json:"node_name"` // 节点名
		}

		type NodeResponse struct {
			Data *NodeConfig `json:"data"`
		}

		var s = NodeConfig{
			IP:       "123",
			NodeName: "132132",
		}
		var sp = NodeResponse{}

		Convey("正常用例", func() {
			err := SetWhenTagValueMatch(&sp, &s, "json", "data")
			So(err, ShouldBeNil)
			So(sp.Data, ShouldNotBeNil)
			So(sp.Data.NodeName, ShouldEqual, "132132")
			So(sp.Data.IP, ShouldEqual, "123")
		})

		Convey("Tag信息为空", func() {
			err := SetWhenTagValueMatch(&sp, &s, "", "")
			So(err, ShouldNotBeNil)
		})

		Convey("当APIObject为空时", func() {
			err := SetWhenTagValueMatch(nil, &s, "json", "data")
			So(err, ShouldNotBeNil)
		})

		Convey("当InternalObject为空时", func() {
			err := SetWhenTagValueMatch(sp, nil, "json", "data")
			So(err, ShouldBeNil)
		})

		Convey("类型不匹配时，应报错", func() {
			wsResponse := &WrongConfig{}
			err := SetWhenTagValueMatch(sp, wsResponse, "json", "data")
			So(err, ShouldNotBeNil)
			So(sp.Data, ShouldBeNil)
		})
	})

	Convey("结构体Tag字段为字符串", t, func() {
		type NodeResponse struct {
			Data string `json:"data"`
		}

		var np = NodeResponse{}

		Convey("正常用例", func() {
			err := SetWhenTagValueMatch(&np, "ok", "json", "data")
			So(err, ShouldBeNil)
			So(np.Data, ShouldEqual, "ok")
		})

		Convey("类型不一致", func() {
			err := SetWhenTagValueMatch(&np, 3234, "json", "data")
			So(err, ShouldNotBeNil)
			So(np.Data, ShouldEqual, "")
		})
	})

	Convey("结构体Tag字段为整形", t, func() {
		type NodeResponse struct {
			Data int `json:"data"`
		}

		var np = NodeResponse{}

		Convey("正常用例", func() {
			err := SetWhenTagValueMatch(&np, 123, "json", "data")
			So(err, ShouldBeNil)
			So(np.Data, ShouldEqual, 123)
		})

		Convey("类型不一致", func() {
			err := SetWhenTagValueMatch(&np, "3234", "json", "data")
			So(err, ShouldNotBeNil)
			So(np.Data, ShouldEqual, 0)
		})
	})

	Convey("结构体Tag字段为布尔值", t, func() {
		type NodeResponse struct {
			Data bool `json:"data"`
		}

		var np = NodeResponse{}

		Convey("正常用例", func() {
			err := SetWhenTagValueMatch(&np, true, "json", "data")
			So(err, ShouldBeNil)
			So(np.Data, ShouldBeTrue)
		})

		Convey("类型不一致", func() {
			err := SetWhenTagValueMatch(&np, "3234", "json", "data")
			So(err, ShouldNotBeNil)
			So(np.Data, ShouldBeFalse)
		})
	})

	Convey("结构体Tag字段为指针", t, func() {
		type NodeResponse struct {
			Data *string `json:"data"`
		}

		var np = NodeResponse{}

		Convey("正常用例", func() {
			d := "haha"
			err := SetWhenTagValueMatch(&np, &d, "json", "data")
			So(err, ShouldBeNil)
			So(*np.Data, ShouldEqual, "haha")
		})

		Convey("类型不一致", func() {
			err := SetWhenTagValueMatch(&np, "3234", "json", "data")
			So(err, ShouldNotBeNil)
			So(np.Data, ShouldBeNil)
		})
	})
}

func TestCheckIfStructFieldMatch(t *testing.T) {
	Convey("检测结构体字段是否匹配", t, func() {
		Convey("值为字符串", func() {
			type Person struct {
				Name string `json:"name"`
			}
			type StructC struct {
				IP     string `json:"ip"` // 节点ip地址
				Digit  int    `json:"digit"`
				Person Person `json:"person"`
			}
			type StructB struct {
				Spec StructC `json:"spec"`
			}

			type StructA struct {
				Data StructB `json:"data"`
			}

			d := StructA{Data: StructB{Spec: StructC{
				IP:     "1111",
				Digit:  9527,
				Person: Person{Name: "test"},
			}}}

			Convey("字段存在", func() {
				Convey("类型一致", func() {
					Convey("字符串", func() {
						Convey("值相同", func() {
							err := CheckIfStructFieldMatch(d, "json", "data.spec.ip", "1111")
							So(err, ShouldBeNil)
						})

						Convey("值不相同", func() {
							err := CheckIfStructFieldMatch(d, "json", "data.spec.ip", "2222")
							So(err, ShouldNotBeNil)
							So(err.Error(), ShouldContainSubstring, "value not match")
						})
					})

					Convey("整型", func() {
						Convey("值相同", func() {
							err := CheckIfStructFieldMatch(d, "json", "data.spec.digit", 9527)
							So(err, ShouldBeNil)
						})

						Convey("值不同", func() {
							err := CheckIfStructFieldMatch(d, "json", "data.spec.digit", 119527)
							So(err, ShouldNotBeNil)
							So(err.Error(), ShouldContainSubstring, "value not match")
						})
					})

					Convey("结构体", func() {
						Convey("值相同", func() {
							err := CheckIfStructFieldMatch(d, "json", "data.spec.person", Person{Name: "test"})
							So(err, ShouldBeNil)
						})

						Convey("值不同", func() {
							err := CheckIfStructFieldMatch(d, "json", "data.spec.person", Person{Name: "dev"})
							So(err, ShouldNotBeNil)
							So(err.Error(), ShouldContainSubstring, "value not match")
						})
					})
				})

				Convey("类型不一致", func() {
					err := CheckIfStructFieldMatch(d, "json", "data.spec.ip", 111)
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldContainSubstring, "value type not match")
				})
			})

			Convey("字段不存在", func() {
				Convey("中间不存在", func() {
					err := CheckIfStructFieldMatch(d, "json", "data.spec2.ip", "1111")
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldContainSubstring, "cannot find field with tag")
				})
				Convey("最终不存在", func() {
					err := CheckIfStructFieldMatch(d, "json", "data.spec.ip2", "1111")
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldContainSubstring, "cannot find field with tag")
				})
			})

		})
	})
}

func TestCheckIfBytesStructFieldMatch(t *testing.T) {
	Convey("TestCheckIfBytesStructFieldMatch", t, func() {
		bytedata := `{"kind":"SubjectAccessReview","apiVersion":"authorization.k8s.io/v1beta1","spec":{"resourceAttributes":{"namespace":"default","verb":"list","version":"v1","resource":"pods"},"user":"wwhvw","group":["system:authenticated","test"],"uid":"wwhvw"},"status":{"allowed":false,"index":55,"float":5.5}}`
		Convey("字段存在", func() {
			Convey("值相同", func() {
				Convey("kind", func() {
					err := CheckIfBytesStructFieldMatch([]byte(bytedata), "kind", "SubjectAccessReview")
					So(err, ShouldBeNil)
				})
				Convey("status.allowed==false", func() {
					err := CheckIfBytesStructFieldMatch([]byte(bytedata), "status.allowed", false)
					So(err, ShouldBeNil)
				})
				Convey("status.index==55", func() {
					err := CheckIfBytesStructFieldMatch([]byte(bytedata), "status.index", 55)
					So(err, ShouldBeNil)
				})
				Convey("status.float==5.5", func() {
					err := CheckIfBytesStructFieldMatch([]byte(bytedata), "status.float", 5.5)
					So(err, ShouldBeNil)
				})
				Convey("spec.resourceAttributes.namespace", func() {
					err := CheckIfBytesStructFieldMatch(
						[]byte(bytedata),
						"spec.resourceAttributes.namespace",
						"default",
					)
					So(err, ShouldBeNil)
				})
				Convey("spec.group比较整个数组", func() {
					valueMap := []string{"system:authenticated", "test"}
					err := CheckIfBytesStructFieldMatch([]byte(bytedata), "spec.group", valueMap)
					So(err, ShouldBeNil)
				})
				Convey("spec.group[1], 索引1号元素比较", func() {
					err := CheckIfBytesStructFieldMatch([]byte(bytedata), "spec.group[1]", "test")
					So(err, ShouldBeNil)
				})
			})
			Convey("值不同", func() {
				Convey("kind!=SubjectAccessReview", func() {
					err := CheckIfBytesStructFieldMatch([]byte(bytedata), "kind", "SubjectAccessReview222")
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldContainSubstring, "value not match")
				})

				Convey("spec.group[1]!=test", func() {
					err := CheckIfBytesStructFieldMatch([]byte(bytedata), "spec.group[1]", "testssss")
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldContainSubstring, "value not match")
				})

				Convey("spec.group[999] 索引不存在", func() {
					err := CheckIfBytesStructFieldMatch([]byte(bytedata), "spec.group[999]", "testssss")
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldContainSubstring, "invalid slice index")
				})

				Convey("status.float!=5.5", func() {
					err := CheckIfBytesStructFieldMatch([]byte(bytedata), "status.float", 0.55)
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldContainSubstring, "value not match")
				})
			})
		})

		Convey("字段不存在", func() {
			err := CheckIfBytesStructFieldMatch([]byte(bytedata), "kind2", "")
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "field not exist")
		})
	})
}

func TestGetBytesStructField(t *testing.T) {
	Convey("TestCheckIfBytesStructFieldMatch", t, func() {
		bytedata := `{"kind":"SubjectAccessReview","apiVersion":"authorization.k8s.io/v1beta1","spec":{"resourceAttributes":{"namespace":"default","verb":"list","version":"v1","resource":"pods"},"user":"wwhvw","group":["system:authenticated","test"],"uid":"wwhvw"},"status":{"allowed":false,"index":55,"float":5.5}}`
		Convey("字符串", func() {
			data, err := GetBytesStructField([]byte(bytedata), "kind")
			So(err, ShouldBeNil)
			So(data, ShouldResemble, "SubjectAccessReview")
		})
		Convey("数组", func() {
			data, err := GetBytesStructField([]byte(bytedata), "spec.group")
			So(err, ShouldBeNil)
			So(data, ShouldResemble, []any{"system:authenticated", "test"})
		})
		Convey("数组元素", func() {
			data, err := GetBytesStructField([]byte(bytedata), "spec.group[1]")
			So(err, ShouldBeNil)
			So(data, ShouldResemble, "test")
		})

		Convey("不存在", func() {
			_, err := GetBytesStructField([]byte(bytedata), "spec.group[10]")
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldEqual, "invalid slice index [10], out of slice range")
		})
	})
}

func TestSetWhenFieldValueTypeMatch(t *testing.T) {

	type Profile struct {
		Name     string     `json:"name"`
		WhatEver int        `json:"what_ever"`
		Time     time.Time  `json:"time"`
		TimeP    *time.Time `json:"time2"`

		//User2 api2.User `json:"user_2"`
		//User2P *api2.User `json:"user_2p"`
	}

	type User struct {
		Id            string `json:"id" description:"这是ID" required:"true" default:""""`
		Name          string
		IdP           *string
		NameP         string
		Bool          bool
		BoolP         *bool
		Int           int
		IntP          *int
		Int32         int32
		Int32P        *int32
		Int64         int64
		Int64P        *int64
		Float32       float32
		Float64       float64
		Float32P      *float32
		Float64P      *float64
		MapString     map[string]string
		MapInt        map[string]int
		MapBool       map[string]bool
		MapMapString  map[string]map[string]string
		MapMapObject  map[string]map[string]Profile
		MapMapObjectP map[string]map[string]*Profile
		MapObject2    map[string]Profile
		MapObjectP    map[string]*Profile
		MapIntP       map[string]*int
		MapBoolP      map[string]*bool
		MapStringP    map[string]*string
		Object        Profile
		ObjectP       *Profile
		ArrayString   []string
		ArrayInt      []int
		ArrayBool     []bool
		ArrayStringP  []*string
		ArrayIntP     []*int
		ArrayBoolP    []*bool
	}
	var u User
	Convey("TestSetWhenFieldValueTypeMatch", t, func() {
		Convey("1", func() {
			Convey("good", func() {
				var err error
				err = SetWhenFieldValueTypeMatch(&u, "Id", "1233")
				So(err, ShouldBeNil)
				So(u.Id, ShouldEqual, "1233")

				err = SetWhenFieldValueTypeMatch(&u, "IdP", &u.Id)
				So(err, ShouldBeNil)
				So(*u.IdP, ShouldEqual, "1233")

				testMap := make(map[string]string)
				testMap["k"] = "v"
				err = SetWhenFieldValueTypeMatch(&u, "MapString", testMap)
				So(err, ShouldBeNil)
				So(u.MapString, ShouldEqual, testMap)
			})

			Convey("bad", func() {
				err := SetWhenFieldValueTypeMatch(&u, "IdP", "1233")
				So(err, ShouldNotBeNil)
			})

		})
	})
}

func TestSetFieldZeroValueIfMatch(t *testing.T) {
	type InnerStruct struct {
		InnerField string
	}

	type MyStruct struct {
		StringField  string
		IntField     int
		PtrField     *InnerStruct
		ArrayField   [2]InnerStruct
		SliceField   []InnerStruct
		MapField     map[string]InnerStruct
		MapPtrField  map[string]*InnerStruct
		ConditionMet string
		Child        *MyStruct
		Child2       *MyStruct
	}

	f := func() MyStruct {
		return MyStruct{
			StringField:  "Hello",
			IntField:     123,
			PtrField:     &InnerStruct{InnerField: "Inner"},
			ArrayField:   [2]InnerStruct{{InnerField: "Array1"}, {InnerField: "Array2"}},
			SliceField:   []InnerStruct{{InnerField: "Slice1"}, {InnerField: "Slice2"}},
			MapField:     map[string]InnerStruct{"key1": {InnerField: "Map1"}},
			MapPtrField:  map[string]*InnerStruct{"key2": {InnerField: "MapPtr1"}},
			ConditionMet: "This should be zeroed",
			Child: &MyStruct{
				StringField: "Hello",
			},
		}
	}

	Convey("FieldName Equal", t, func() {
		SkipConvey("1st field", func() {
			// Initialize the struct
			myStruct := f()
			SetFieldZeroValueIfCondition(&myStruct, func(s string) bool {
				return s == "ConditionMet" || s == "StringField" || s == "IntField"
			})
			So(myStruct.ConditionMet, ShouldEqual, "")
			So(myStruct.IntField, ShouldEqual, 0)
			So(myStruct.StringField, ShouldEqual, "")
		})
		SkipConvey("sub field", func() {
			myStruct := f()
			SetFieldZeroValueIfCondition(&myStruct, func(s string) bool {
				return s == "InnerField"
			})
			So(myStruct.ConditionMet, ShouldNotEqual, "")
			So(myStruct.PtrField.InnerField, ShouldEqual, "")
			So(myStruct.ArrayField[0].InnerField, ShouldEqual, "")
			So(myStruct.SliceField[0].InnerField, ShouldEqual, "")
			//So(myStruct.MapField["key1"].InnerField, ShouldEqual, "")
			So(myStruct.MapPtrField["key2"].InnerField, ShouldEqual, "")

			So(myStruct.Child.StringField, ShouldEqual, "")
		})
		Convey("sub sub field", func() {
			myStruct := f()
			SetFieldZeroValueIfCondition(&myStruct, func(s string) bool {
				return s == "StringField"
			})
			So(myStruct.StringField, ShouldEqual, "")
			So(myStruct.Child.StringField, ShouldEqual, "")
		})
	})

}
