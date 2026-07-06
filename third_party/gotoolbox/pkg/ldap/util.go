package ldap

import (
	"fmt"
	"strings"
	"time"

	ldap "github.com/go-ldap/ldap/v3"
	"github.com/sirupsen/logrus"
	"github.com/wangweihong/gotoolbox/pkg/stringutil"
)

const (
	createTimeAttr         = "whenCreated"     // AD time attribute
	createTimeOpenLdapAttr = "createTimestamp" //openLDAP 2.4 Use createTimestamp attribute as create time
)

func getAttrs(e ldap.Entry, name string) []string {
	for _, a := range e.Attributes {
		if a.Name != name {
			continue
		}
		return a.Values
	}
	// DN/dn值为ldap entry唯一ID.不作为属性值存在
	if name == "DN" || name == "dn" {
		return []string{e.DN}
	}

	return nil
}

func getAttr(e ldap.Entry, name string) string {
	if name == "" {
		return ""
	}
	if a := getAttrs(e, name); len(a) > 0 {
		return a[0]
	}
	return ""
}

func userSearchAttributes(us UserSearch) []string {
	var attribute []string
	if us.UIDAttr != "" {
		attribute = append(attribute, us.UIDAttr)
	}

	if us.UserNameAttr != "" {
		attribute = append(attribute, us.UserNameAttr)
	}

	if us.EmailAttr != "" {
		attribute = append(attribute, us.EmailAttr)
	}

	if us.PhoneAttr != "" {
		attribute = append(attribute, us.PhoneAttr)
	}

	//  如果什么属性都不指定, 可能要求返回全部属性
	if attribute != nil {
		attribute = append(attribute, createTimeOpenLdapAttr)
		attribute = append(attribute, createTimeAttr)
	}

	return attribute
}

func entryToAccount(us UserSearch, entry ldap.Entry) Account {
	var account Account
	account.Name = getAttr(entry, us.UserNameAttr)
	account.UUID = getAttr(entry, us.UIDAttr)
	account.Email = getAttr(entry, us.EmailAttr)
	account.Phone = getAttr(entry, us.PhoneAttr)
	account.DN = entry.DN

	createTimeString := getAttr(entry, createTimeAttr)
	createTimeOpenLdap := getAttr(entry, createTimeOpenLdapAttr)

	// windows AD域和openldap采用的时间戳格式不同。
	var err error
	if createTimeString != "" {
		account.CreateTime, err = time.Parse("20060102150405.0Z", createTimeString)
		if err != nil {
			logrus.Debugf("Parse time %v fail: %v", createTimeString, err)
		}
	}
	if createTimeOpenLdap != "" {
		account.CreateTime, err = time.Parse("20060102150405Z", createTimeOpenLdap)
		if err != nil {
			logrus.Debugf("Parse time with time format \"20060102150405Z\" %v fail: %v", createTimeOpenLdap, err)
		}
	}
	return account
}

func groupSearchAttributes(us GroupSearch) []string {
	var attribute []string

	if us.NameAttr != "" {
		attribute = append(attribute, us.NameAttr)
	}

	if us.MemberAttr != "" {
		attribute = append(attribute, us.MemberAttr)
	}

	//  如果什么属性都不指定, 可能要求返回全部属性
	if attribute != nil {
		attribute = append(attribute, createTimeOpenLdapAttr)
		attribute = append(attribute, createTimeAttr)
	}
	return attribute
}

func entryToGroup(us GroupSearch, entry ldap.Entry) Group {
	var account Group

	account.Name = getAttr(entry, us.NameAttr)
	account.DN = entry.DN
	createTimeString := getAttr(entry, createTimeAttr)
	createTimeOpenLdap := getAttr(entry, createTimeOpenLdapAttr)

	// windows AD域和openldap采用的时间戳格式不同。
	var err error
	if createTimeString != "" {
		account.CreateTime, err = time.Parse("20060102150405.0Z", createTimeString)
		if err != nil {
			logrus.Debugf("Parse time %v fail: %v", createTimeString, err)
		}
	}
	if createTimeOpenLdap != "" {
		account.CreateTime, err = time.Parse("20060102150405Z", createTimeOpenLdap)
		if err != nil {
			logrus.Debugf("Parse time with time format \"20060102150405Z\" %v fail: %v", createTimeOpenLdap, err)
		}
	}

	account.members = getAttrs(entry, us.MemberAttr)
	return account
}

// for example.
// 0. origin:a --> a;
// 1. origin:(objectClass=person) -- > (objectClass=person)
// 2. origin:(objectClass=person)(uid=*) --> (&(objectClass=person)(uid=*))
// 3. origin:(objectClass=person) other: name, department=aa --> (&(objectClass=person)(name=*)(department=aa))
// 3. origin:(objectClass=person) other: (department=aa) --> (&(objectClass=person)(department=aa))
// 3. origin:(objectClass=person) other: (name=*)(department=aa) --> (&(objectClass=person)(name=*)(department=aa))
func filterToLdapSearchFilter(origin string, others ...string) string {
	// invalid search filter, ignore convert
	if !strings.HasPrefix(origin, "(") || !strings.HasSuffix(origin, ")") ||
		strings.HasPrefix(origin, "(&") {
		return origin
	}

	multiple := stringutil.SubStringNums(origin, "(") > 1
	filter := origin
	for _, o := range others {
		multiple = true

		if !strings.HasPrefix(o, "(") || !strings.HasSuffix(o, ")") {
			kv := strings.SplitN(o, "=", 2)
			//DN或者dn无法作为过滤条件
			//dn=*无法查询到有效的entry
			if kv[0] == "dn" || kv[0] == "DN" {
				continue
			}
			v := "*"
			if len(kv) == 2 {
				v = kv[1]
			}
			o = fmt.Sprintf("(%v=%v)", kv[0], v)
		}

		filter = fmt.Sprintf("%s%s", filter, o)
	}

	if multiple {
		filter = fmt.Sprintf("(&%s)", filter)
	}
	return filter
}
