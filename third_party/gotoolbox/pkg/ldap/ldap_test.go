package ldap

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-ldap/ldap/v3"
	. "github.com/smartystreets/goconvey/convey"
)

var (
	existUserName          = "jane"
	existUserWrongPassword = "wrong"
	existUserRightPassword = "foo"
	lcAllOK                = Config{
		//		Addr:       "192.168.134.128",
		Addr:       "10.30.61.234",
		Port:       389,
		BindDN:     "cn=admin,dc=example,dc=org",
		BindPasswd: "admin",
		EnableTls:  false,
	}

	us = UserSearch{
		BaseDN:       "ou=People,dc=example,dc=org",
		UserNameAttr: "cn",
		UIDAttr:      "dn",
		EmailAttr:    "mail",
		Filter:       "(objectClass=person)",
	}

	gs = GroupSearch{
		BaseDN: "ou=Groups,dc=example,dc=org",
		Filter: "(objectClass=groupOfNames)",

		NameAttr:   "cn",
		MemberAttr: "member",
	}

	ugm = UserGroupMatcher{

		UserAttr:  "dn",
		GroupAttr: "member",
	}

	ugs = UserGroupSearch{
		Search:  us,
		Matcher: ugm,
	}

	gus = GroupUserSearch{
		Search:  gs,
		Matcher: ugm,
	}
)

func TestConnectLDAP(t *testing.T) {
	t.Run("correct config connect ldap", func(t *testing.T) {
		if _, err := connectLDAP(lcAllOK, 0); err != nil {
			t.Fatalf("good ldap config connect fail:%v.", err)
		}
		t.Logf("good ldap config %+v connect success", lcAllOK)
	})

}

func TestSyncAccounts(t *testing.T) {
	Convey("用户同步", t, func() {
		Convey("全部用户同步", func() {
			Convey("不返回用户组", func() {
				accounts, _, err := SyncAccounts(context.Background(), lcAllOK, us, nil, 0)
				So(err, ShouldBeNil)
				Printf("accounts:%v\n", len(accounts))
				for _, v := range accounts {
					So(v.Group, ShouldBeNil)
				}
			})
			SkipConvey("返回用户组", func() {
				accounts, _, err := SyncAccounts(context.Background(), lcAllOK, us, &gus, 0)
				So(err, ShouldBeNil)
				Printf("accounts:%v\n", len(accounts))
				for _, v := range accounts {
					Printf("account:%v,group:%v\n", v.Name, v.Group)
				}
			})

		})

		SkipConvey("指定用户同步,可能返回多个，取决于属性", func() {
			Convey("不返回用户组", func() {
				us := us
				us.Filter = "(cn=user1000)"
				accounts, _, err := SyncAccounts(context.Background(), lcAllOK, us, nil, 0)
				So(err, ShouldBeNil)
				So(len(accounts), ShouldEqual, 1)
				So(accounts[0].Group, ShouldBeNil)
			})
			Convey("返回用户组", func() {
				us := us
				us.Filter = "(cn=user1000)"
				accounts, _, err := SyncAccounts(context.Background(), lcAllOK, us, &gus, 0)
				So(err, ShouldBeNil)
				So(len(accounts), ShouldEqual, 1)
				So(accounts[0].Group, ShouldNotBeNil)
			})
		})

	})

}

func TestSyncGroups(t *testing.T) {
	Convey("用户组同步", t, func() {
		Convey("不同步用户", func() {
			SkipConvey("指定Class用户组同步", func() {
				accounts, err := SyncGroups(context.Background(), lcAllOK, gs, nil, 0)
				So(err, ShouldBeNil)
				Println("group num:", len(accounts))
			})
			Convey("获取指定用户的用户组", func() {
				Convey("未指定用户组成员属性时,通过过滤也能拿到指定用户john所在的用户组", func() {
					gs := gs
					gs.MemberAttr = ""
					gs.Filter = filterToLdapSearchFilter(gs.Filter, fmt.Sprintf("(member=%s)", ldap.EscapeFilter("cn=john,ou=People,dc=example,dc=org")))
					Printf("filter:%v\n", gs.Filter)
					accounts, err := SyncGroups(context.Background(), lcAllOK, gs, nil, 0)
					So(err, ShouldBeNil)
					Println("group num:", len(accounts))
				})

				SkipConvey("获取所有具有member属性的用户组", func() {
					gs := gs
					gs.MemberAttr = ""
					gs.Filter = filterToLdapSearchFilter(gs.Filter, fmt.Sprintf("(member=%s)", "*"))
					accounts, err := SyncGroups(context.Background(), lcAllOK, gs, nil, 0)
					So(err, ShouldBeNil)
					Println("group num:", len(accounts))
				})

				SkipConvey("过滤条件带有用户组不具备的某个属性", func() {
					gs := gs
					gs.MemberAttr = ""
					gs.Filter = filterToLdapSearchFilter(gs.Filter, fmt.Sprintf("(noexist=%s)", "*"))
					accounts, err := SyncGroups(context.Background(), lcAllOK, gs, nil, 0)
					So(err, ShouldBeNil)
					So(len(accounts), ShouldEqual, 0)
				})
			})
		})
	})
}

func TestSearchEntriesWithPaging(t *testing.T) {
	Convey("原始搜索", t, func() {
		Convey("组搜索", func() {
			Convey("组搜索全部", func() {
				dn := gs.BaseDN
				filter := "(objectClass=*)"
				accounts, err := SearchEntriesWithPaging(context.Background(), lcAllOK, dn, nil, filter, 0)
				So(err, ShouldBeNil)
				for _, v := range accounts {
					Printf("dn:%v\n", v.DN)
					for _, j := range v.Attributes {
						Printf("---> attr %v: %v\n", j.Name, j.Values)
					}
				}
			})
			Convey("按组名搜索", func() {
				dn := gs.BaseDN
				filter := "(&(objectClass=groupOfNames)(cn=group1))"
				accounts, err := SearchEntriesWithPaging(context.Background(), lcAllOK, dn, nil, filter, 0)
				So(err, ShouldBeNil)
				for _, v := range accounts {
					Printf("dn:%v\n", v.DN)
					for _, j := range v.Attributes {
						Printf("---> attr %v: %v\n", j.Name, j.Values)
					}
				}
			})

			Convey("搜索包含指定用户dn的组", func() {
				dn := gs.BaseDN
				filter := "(&(objectClass=groupOfNames)(member=cn=user1000,ou=People,dc=example,dc=org))"
				accounts, err := SearchEntriesWithPaging(context.Background(), lcAllOK, dn, nil, filter, 0)
				So(err, ShouldBeNil)
				for _, v := range accounts {
					Printf("dn:%v\n", v.DN)
					for _, j := range v.Attributes {
						Printf("---> attr %v: %v\n", j.Name, j.Values)
					}
				}
			})
		})

		Convey("用户 同步", func() {
			Convey("单个用户同步", func() {
				dn := us.BaseDN
				filter := "(cn=user1000)"
				accounts, err := SearchEntriesWithPaging(context.Background(), lcAllOK, dn, nil, filter, 0)
				So(err, ShouldBeNil)
				for _, v := range accounts {
					Printf("dn:%v\n", v.DN)
					for _, j := range v.Attributes {
						Printf("---> attr %v: %v\n", j.Name, j.Values)
					}
				}
			})
			//cn=jane,ou=People,dc=example,dc=org
			Convey("单个用户使用dn同步不返回有效的索引", func() {
				dn := us.BaseDN
				filter := "(dn=cn=jane,ou=People,dc=example,dc=org)"
				accounts, err := SearchEntriesWithPaging(context.Background(), lcAllOK, dn, nil, filter, 0)
				So(err, ShouldBeNil)
				So(len(accounts), ShouldEqual, 0)
			})
		})
	})
}

func TestFilterToLdapSearchFilter(t *testing.T) {
	Convey("过滤转换", t, func() {
		Convey("非括号正确包裹字符串", func() {
			So(filterToLdapSearchFilter("aaa"), ShouldEqual, "aaa")
			So(filterToLdapSearchFilter("aaa", "bbb"), ShouldEqual, "aaa")
			So(filterToLdapSearchFilter("aaa)"), ShouldEqual, "aaa)")
			So(filterToLdapSearchFilter("aaa)", "bbb"), ShouldEqual, "aaa)")
			So(filterToLdapSearchFilter("(aaa"), ShouldEqual, "(aaa")
			So(filterToLdapSearchFilter("(aaa", "bbb"), ShouldEqual, "(aaa")
			So(filterToLdapSearchFilter(")aa(a"), ShouldEqual, ")aa(a")
			So(filterToLdapSearchFilter(")aa(a", "bbb"), ShouldEqual, ")aa(a")
			So(filterToLdapSearchFilter("aa()a"), ShouldEqual, "aa()a")
			So(filterToLdapSearchFilter("aa()a", "bbb"), ShouldEqual, "aa()a")
		})

		Convey("单括号正确包裹字符串", func() {
			So(filterToLdapSearchFilter("(user=aa)"), ShouldEqual, "(user=aa)")
			So(filterToLdapSearchFilter("(user=aa)", "bbb"), ShouldEqual, "(&(user=aa)(bbb=*))")
			So(filterToLdapSearchFilter("(user=aa)", "group=alpha"), ShouldEqual, "(&(user=aa)(group=alpha))")
			So(filterToLdapSearchFilter("(user=aa)", "group=alpha", "team"), ShouldEqual, "(&(user=aa)(group=alpha)(team=*))")
		})

		Convey("多括号包裹", func() {
			So(filterToLdapSearchFilter("(user=aa)(team=dev)"), ShouldEqual, "(&(user=aa)(team=dev))")
			So(filterToLdapSearchFilter("(user=aa)(team=dev)", "bbb"), ShouldEqual, "(&(user=aa)(team=dev)(bbb=*))")
		})

		Convey("other", func() {
			So(filterToLdapSearchFilter("(team=dev)", "name"), ShouldEqual, "(&(team=dev)(name=*))")
			So(filterToLdapSearchFilter("(team=dev)", "name=aa"), ShouldEqual, "(&(team=dev)(name=aa))")
			So(filterToLdapSearchFilter("(team=dev)", "(name=aa)"), ShouldEqual, "(&(team=dev)(name=aa))")
			So(filterToLdapSearchFilter("(team=dev)", "(name=aa)(zone=beijing)"), ShouldEqual, "(&(team=dev)(name=aa)(zone=beijing))")
		})

	})
}

func TestAuthenticate(t *testing.T) {
	Convey("验证", t, func() {
		Convey("不带用户组", func() {
			Convey("用户名进行登录验证", func() {
				Convey("正确认证", func() {
					account, err := Authentication(context.Background(), lcAllOK, us, "cn", "user1000", "user1000", nil, 0)
					So(err, ShouldBeNil)
					So(account.Group, ShouldBeNil)
				})

				Convey("错误密码", func() {
					_, err := Authentication(context.Background(), lcAllOK, us, "cn", "user1000", "wrong", nil, 0)
					So(err, ShouldNotBeNil)
				})

				Convey("不存在的用户", func() {
					_, err := Authentication(context.Background(), lcAllOK, us, "cn", "user10002s45", "user1000", nil, 0)
					So(err, ShouldNotBeNil)
					//So(err,ShouldEqual,errAuthenticateFail)
				})

			})

			Convey("邮箱名进行登录验证", func() {
				Convey("正确认证", func() {
					account, err := Authentication(context.Background(), lcAllOK, us, "mail", "user1000@example.com", "user1000", nil, 0)
					So(err, ShouldBeNil)
					So(account.Group, ShouldBeNil)
				})

				Convey("错误密码", func() {
					_, err := Authentication(context.Background(), lcAllOK, us, "mail", "user1000@example.com", "wrong", nil, 0)
					So(err, ShouldNotBeNil)
				})

				Convey("不存在的用户", func() {
					_, err := Authentication(context.Background(), lcAllOK, us, "mail", "user10002s45", "user1000", nil, 0)
					So(err, ShouldNotBeNil)
				})
			})

			Convey("不存在的属性", func() {
				_, err := Authentication(context.Background(), lcAllOK, us, "whatever", "user1000@example.com", "user1000", nil, 0)
				So(err, ShouldNotBeNil)
			})
		})

		Convey("带用户组", func() {
			account, err := Authentication(context.Background(), lcAllOK, us, "cn", "user1000", "user1000", &gus, 0)
			So(err, ShouldBeNil)
			So(account.Group, ShouldNotBeNil)
		})
	})
}

func TestSyncGroupAndAccounts(t *testing.T) {
	Convey("同步用户组和账号", t, func() {
		Convey("只同步账号", func() {
			g, u, err := SyncGroupsAccounts(context.Background(), lcAllOK, &us, nil, nil, 30*time.Second)
			So(err, ShouldBeNil)
			So(g, ShouldBeNil)
			So(u, ShouldNotBeNil)
			So(u[0].Group, ShouldBeNil)
		})

		Convey("只同步组", func() {
			g, u, err := SyncGroupsAccounts(context.Background(), lcAllOK, nil, &gs, nil, 30*time.Second)
			So(err, ShouldBeNil)
			So(g, ShouldNotBeNil)
			So(g[0].Members, ShouldBeNil)
			So(u, ShouldBeNil)
		})

		Convey("用户和组都同步,但不匹配", func() {
			g, u, err := SyncGroupsAccounts(context.Background(), lcAllOK, &us, &gs, nil, 30*time.Second)
			So(err, ShouldBeNil)
			So(g, ShouldNotBeNil)
			So(g[0].Members, ShouldBeNil)
			So(u, ShouldNotBeNil)
			So(u[0].Group, ShouldBeNil)
		})

		Convey("用户和组都同步且匹配", func() {
			g, u, err := SyncGroupsAccounts(context.Background(), lcAllOK, &us, &gs, &ugm, 30*time.Second)
			So(err, ShouldBeNil)
			So(g, ShouldNotBeNil)
			So(g[1].Members, ShouldNotBeNil)
			So(g[0].Members, ShouldNotBeNil)
			So(u, ShouldNotBeNil)
			So(u[0].Group, ShouldNotBeNil)
		})
	})
}
