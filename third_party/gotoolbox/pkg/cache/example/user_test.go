package example_test

import (
	"testing"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/cache/example"
	"github.com/wangweihong/gotoolbox/pkg/sets"

	. "github.com/smartystreets/goconvey/convey"
)

func TestUserList(t *testing.T) {
	um := example.NewUMInstance()
	um.Add(&example.User{
		Name:       "aaa",
		UUID:       "user1",
		Tenant:     "tenant1",
		Group:      []string{"group1", "group2"},
		Roles:      []string{"role1", "role2"},
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	})

	um.Add(&example.User{
		Name:       "aaa",
		UUID:       "user2",
		Tenant:     "tenant2",
		Group:      []string{"group1", "group3"},
		Roles:      []string{"role1", "role2"},
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	})

	Convey("用户列表", t, func() {
		Convey("查询某个组用户列表", func() {
			Convey("不存在的用户组", func() {
				So(len(um.ListInGroup("notexist")), ShouldEqual, 0)
			})
			Convey("存在的用户组", func() {
				g1 := um.ListInGroup("group1")
				So(len(g1), ShouldEqual, 2)
				for _, v := range g1 {
					So(sets.NewString(v.Group...).Has("group1"), ShouldBeTrue)
				}
			})
			Convey("ListInGroup应和ListInGroupIndex一致", func() {
				us := um.ListInGroup("group1")
				So(len(us), ShouldNotEqual, 0)

				usIndex := um.ListInGroupIndex("group1")
				So(len(usIndex), ShouldNotEqual, 0)
			})
		})
		Convey("查询某个租户用户列表键", func() {
			t1 := um.ListInTenantIndex("tenant1")
			So(len(t1), ShouldEqual, 1)
		})
	})

}

func TestUserManager_Add(t *testing.T) {
	um := example.NewUMInstance()

	Convey("用户添加", t, func() {
		u1 := &example.User{
			Name:       "aaa",
			Tenant:     "tenant1",
			Group:      []string{"group1", "group2"},
			Roles:      []string{"role1", "role2"},
			CreateTime: time.Now(),
			UpdateTime: time.Now(),
		}
		nu1, err := um.Add(u1)
		So(err, ShouldBeNil)
		So(nu1.UUID, ShouldNotBeEmpty)

		//"入参数据改变不影响缓存数据"
		u1.Name = "bbb"

		nu2, exists, err := um.Get(nu1)
		So(err, ShouldBeNil)
		So(exists, ShouldBeTrue)
		So(nu2.Name, ShouldNotEqual, u1.Name)

		//"添加返回数据改变不影响缓存数据"
		nu1.Name = "bbb"
		nu3, exists, err := um.Get(nu1)
		So(err, ShouldBeNil)
		So(exists, ShouldBeTrue)
		So(nu3.Name, ShouldNotEqual, nu1.Name)
	})

}

func TestUserManager_CleanGroup(t *testing.T) {
	um := example.NewUMInstance()

	Convey("清除用户中某个组的索引", t, func() {
		u1 := &example.User{
			Name:       "aaa",
			Tenant:     "tenant1",
			Group:      []string{"group1", "group2"},
			Roles:      []string{"role1", "role2"},
			CreateTime: time.Now(),
			UpdateTime: time.Now(),
		}
		nu1, err := um.Add(u1)
		So(err, ShouldBeNil)
		So(nu1.UUID, ShouldNotBeEmpty)

		Convey("清除组group1", func() {
			us := um.ListInGroup("group1")
			So(len(us), ShouldNotEqual, 0)

			usIndex := um.ListInGroupIndex("group1")
			So(len(usIndex), ShouldNotEqual, 0)

			So(len(us), ShouldEqual, len(usIndex))

			err = um.CleanGroup("group1")
			So(err, ShouldBeNil)

			us = um.ListInGroup("group1")
			So(len(us), ShouldEqual, 0)
		})
	})
}

func TestUserManager_CleanRole(t *testing.T) {
	um := example.NewUMInstance()
	Convey("清除用户中某个角色的索引", t, func() {
		u1 := &example.User{
			Name:       "aaa",
			Tenant:     "tenant1",
			Group:      []string{"group1", "group2"},
			Roles:      []string{"role1", "role2"},
			CreateTime: time.Now(),
			UpdateTime: time.Now(),
		}
		nu1, err := um.Add(u1)
		So(err, ShouldBeNil)
		So(nu1.UUID, ShouldNotBeEmpty)

		Convey("清除角色role1", func() {
			us := um.ListInRole("role1")
			So(len(us), ShouldNotEqual, 0)

			usIndex := um.ListInRoleIndex("role1")
			So(len(usIndex), ShouldNotEqual, 0)

			So(len(us), ShouldEqual, len(usIndex))

			err = um.CleanRole("role1")
			So(err, ShouldBeNil)

			us = um.ListInRole("role1")
			So(len(us), ShouldEqual, 0)

		})
	})
}
