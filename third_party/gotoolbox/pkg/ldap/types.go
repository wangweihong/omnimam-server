package ldap

import (
	"fmt"
	"time"
)

var (
	errFilterEntriesEmpty = fmt.Errorf("search with filter has no users")
	errAuthenticateFail   = fmt.Errorf("authenticate fail:Invalid Credentials")
)

type Account struct {
	UUID       string
	Name       string
	Email      string
	CreateTime time.Time
	Phone      string
	Password   string
	Group      []string
	DN         string
}

type Group struct {
	CreateTime time.Time
	DN         string
	Name       string
	UUID       string
	Members    []string

	// 特别注意 member字段中存放的是具体是用户哪种属性,取决于Ldap服务器的设置
	members []string
}

// ldap config
type Config struct {
	Addr       string // 地址
	Port       int    // 端口
	BindDN     string // 用户名
	BindPasswd string // 密码
	EnableTls  bool   // 启动TLS
}

// user search condition
type UserSearch struct {
	// BaseDN: LDAP基本域，如openldap:cn=admin,dc=example,dc=com; windows AD域：OU=test,DC=example,DC=org
	// 不同对象类型的基本域不同。
	BaseDN string

	// UserNameAttr: 用户名如sAMAccountName,具体根据根据ldap服务器的配置. openldap用cn来表示
	UserNameAttr string

	// UIDAttr: 用户在LDAP服务器的唯一ID(即DN)的属性字段. UIDAttr通常为<用户名属性字段>=用户名+BaseDN.
	//		    如openldap,用户foo的DN为cn=foo,cn=admin,dc=example,dc=com
	// 			windows AD和open LDAP可能不同。值可能是DN，也可能是dn
	UIDAttr string

	// EmailAttr: 邮箱属性字段
	EmailAttr string // 邮箱属性

	// PhoneAttr: 手机号属性
	PhoneAttr string // 手机号

	// Filter: 过滤字段如`(objectClass=person)`。用于过滤掉非用户对象(如组对象`(objectClass=groupOfNames)`)。
	// 		       ldap中新建一个usergroup其也可能包含用户指定的UIDAttr字段，通过增加过滤字段可以精确的过滤到用户
	//			   通常也可以还会结合用户名或者邮箱账号来精确定位到某个用户
	Filter string
}

type GroupSearch struct {
	// BaseDN to start the search from. For example "cn=groups,dc=example,dc=com"
	BaseDN string `json:"baseDN"`

	// Optional filter to apply when searching the directory. For example "(objectClass=posixGroup)"
	Filter string `json:"filter"` // 组对象过滤字段

	// The attribute of the group that represents its name.
	NameAttr string `json:"nameAttr"` //组名字段。 如果组名唯一，可根据组名+Filter定位到唯一的组DN.

	MemberAttr string `json:"memberAttr"` // 用户在用户组的字段属性
}

// UserMatcher holds information about user and group matching.
/*
用户数据
dn: cn=jane,ou=People,dc=example,dc=org  # 用户的唯一ID。 LDAP通过该ID来唯一标志
objectClass: person # 用户对象类
objectClass: inetOrgPerson
sn: doe   # 用户姓，这些属性不重要
cn: jane  # 用户名
mail: janedoe@example.com
userpassword: foo

# 用户组数据
dn: cn=admins,ou=Groups,dc=example,dc=org  # 用户组唯一ID
objectClass: groupOfNames # 组对象类。通过这个字段来判断某个dn是用户，还是用户组
cn: admins # 用户组名
member: cn=john,ou=People,dc=example,dc=org  # 用户组中成员列表。存放的是用户的dn属性
member: cn=jane,ou=People,dc=example,dc=org

上述例子中UserGroupMatcher值为：
{
	userAttr:  "dn",
	groupAttr: "member",
}


*/
type UserGroupMatcher struct {
	UserAttr  string `json:"userAttr"`  // 用户组中成员列表属性字段中存放的是用户的哪一个属性。上例中member的值为用户dn的数据,则值为"dn"
	GroupAttr string `json:"groupAttr"` // 用户组哪个属性字段存放组成员列表。如上例中"member"字段。这里则是"member"
}

type UserGroupSearch struct {
	Search  UserSearch       `json:"userSearch"`
	Matcher UserGroupMatcher `json:"matcher"`
}

type GroupUserSearch struct {
	Search  GroupSearch
	Matcher UserGroupMatcher `json:"matcher"`
}
