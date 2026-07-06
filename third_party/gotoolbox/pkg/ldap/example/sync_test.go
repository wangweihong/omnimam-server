package example

import (
	"context"
	"github.com/wangweihong/gotoolbox/pkg/ldap"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

var (
	lcAllOK = ldap.Config{
		Addr:       "192.168.134.128",
		Port:       389,
		BindDN:     "cn=admin,dc=example,dc=org",
		BindPasswd: "admin",
		EnableTls:  false,
	}

	us = ldap.UserSearch{
		BaseDN:       "ou=People,dc=example,dc=org",
		UserNameAttr: "cn",
		UIDAttr:      "dn",
		EmailAttr:    "mail",
		Filter:       "(objectClass=person)",
	}

	gs = ldap.GroupSearch{
		BaseDN: "ou=Groups,dc=example,dc=org",
		Filter: "(objectClass=groupOfNames)",

		NameAttr:   "cn",
		MemberAttr: "member",
	}

	ugm = ldap.UserGroupMatcher{
		UserAttr:  "dn",
		GroupAttr: "member",
	}
)

func TestSyncLdapUser(t *testing.T) {

	Convey("同步ldap账号到缓存", t, func() {
		SkipConvey("缓存中无ldap用户", func() {
			CacheInstance.Clean()
			CacheInstance.CreateUser(&User{
				Name:     "jane",
				Password: "jane",
			})
			CacheInstance.CreateUser(&User{
				Name:     "jane2",
				Password: "jane2",
			})
			CacheInstance.CreateUser(&User{
				Name:     "john",
				Password: "john",
			})
			CacheInstance.CreateGroup(&Group{
				Name: "developers",
			})

			CacheInstance.Print()

			result, err := SyncLdapUser(context.Background(), &lcAllOK, us, &gs, &ugm)
			So(err, ShouldBeNil)
			So(result.Total, ShouldNotEqual, 0)
			So(result.Fail, ShouldEqual, 0)
			So(len(CacheInstance.ListGroups()), ShouldEqual, 3)
			So(len(CacheInstance.ListUsers()), ShouldEqual, 5)
			for _, v := range CacheInstance.ListUsers() {
				if v.IsLDAP && v.Name == "jane" {
					So(len(v.UserGroup), ShouldEqual, 2)
				}

				if v.IsLDAP && v.Name == "john" {
					So(len(v.UserGroup), ShouldEqual, 1)
				}
			}
			CacheInstance.Print()

		})
		SkipConvey("ldap用户已经被删除, 同步后应删除本地缓存", func() {
			CacheInstance.Clean()

			CacheInstance.CreateUser(&User{
				Name:     "jane2",
				Password: "jane2",
				IsLDAP:   true,
			})
			CacheInstance.CreateUser(&User{
				Name:     "jane3",
				Password: "jane3",
				IsLDAP:   true,
			})
			CacheInstance.CreateUser(&User{
				Name:     "jane4",
				Password: "jane4",
				IsLDAP:   true,
			})

			CacheInstance.Print()

			result, err := SyncLdapUser(context.Background(), &lcAllOK, us, &gs, &ugm)
			So(err, ShouldBeNil)
			So(result.Total, ShouldNotEqual, 0)
			So(result.Fail, ShouldEqual, 0)
			So(len(CacheInstance.ListUsers()), ShouldEqual, 2)
			var ldapUserNames []string
			for _, v := range CacheInstance.ListUsers() {
				if v.IsLDAP {
					ldapUserNames = append(ldapUserNames, v.Name)
				}
			}
			So(ldapUserNames, ShouldNotContain, "jane2")
			So(ldapUserNames, ShouldNotContain, "jane3")
			So(ldapUserNames, ShouldNotContain, "jane4")

			CacheInstance.Print()

		})
		Convey("ldap用户已存在本地时，应更新组关系", func() {
			CacheInstance.Clean()

			CacheInstance.CreateUser(&User{
				Name:     "jane",
				Password: "jane",
				IsLDAP:   true,
			})

			CacheInstance.Print()
			for _, v := range CacheInstance.ListUsers() {
				if v.IsLDAP && v.Name == "jane" {
					So(len(v.UserGroup), ShouldEqual, 0)
				}
			}

			result, err := SyncLdapUser(context.Background(), &lcAllOK, us, &gs, &ugm)
			So(err, ShouldBeNil)
			So(result.Total, ShouldNotEqual, 0)
			So(result.Fail, ShouldEqual, 0)
			So(len(CacheInstance.ListUsers()), ShouldEqual, 2)

			for _, v := range CacheInstance.ListUsers() {
				if v.IsLDAP && v.Name == "jane" {
					So(len(v.UserGroup), ShouldNotEqual, 0)
				}
			}

			CacheInstance.Print()

		})

	})
}
