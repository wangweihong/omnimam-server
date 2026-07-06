package ldap

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	ldap "github.com/go-ldap/ldap/v3"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/sets"
)

func connectLDAP(config Config, dialTimeout time.Duration) (*ldap.Conn, error) {
	if dialTimeout == 0 {
		dialTimeout = 60 * time.Second
	}

	dailer := &net.Dialer{Timeout: dialTimeout}

	url := "ldap://" + config.Addr + ":" + strconv.Itoa(config.Port)
	c, err := ldap.DialURL(url, ldap.DialWithDialer(dailer))
	if err != nil {
		return nil, errors.Errorf("ldap dial url %v fail:%v", url, err)
	}

	//FIXME: support TLS certificate
	if config.EnableTls {
		if err := c.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil {
			return nil, err
		}
	}

	return c, nil
}

func _do(config Config, dialTimeout time.Duration, call func(c *ldap.Conn) error) error {
	conn, err := connectLDAP(config, dialTimeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	if config.BindDN == "" && config.BindPasswd == "" {
		if err := conn.UnauthenticatedBind(""); err != nil {
			return errors.Errorf("ldap: initial anonymous bind failed: %v", err)
		}
	} else {
		if err := conn.Bind(config.BindDN, config.BindPasswd); err != nil {
			return errors.Errorf("ldap: initial bind for user %q failed: %v", config.BindDN, err)
		}
	}

	return call(conn)
}

func SyncAccounts(ctx context.Context, config Config, userSearch UserSearch, gus *GroupUserSearch, dialTimeout time.Duration) ([]Account, []*ldap.Entry, error) {
	if userSearch.BaseDN == "" {
		return nil, nil, errors.Errorf("missing user baseDN")
	}

	if userSearch.Filter == "" {
		return nil, nil, errors.Errorf("missing user search filter")
	}

	if userSearch.UserNameAttr == "" {
		return nil, nil, errors.Errorf("missing user name attribute")
	}

	userAttributes := userSearchAttributes(userSearch)
	userFilter := filterToLdapSearchFilter(userSearch.Filter)
	userEntries, err := SearchEntriesWithPaging(ctx, config, userSearch.BaseDN, userAttributes, userFilter, dialTimeout)
	if err != nil {
		return nil, nil, errors.Errorf("ldap: search with paging fail:%v", err.Error())
	}
	accounts := make([]Account, 0)
	var processed bool
	// 如果要求同时返回用户的用户组信息
	if gus != nil {
		// TODO: check param
		//确保返回member属性
		gus.Search.MemberAttr = gus.Matcher.GroupAttr
		groupAttributes := groupSearchAttributes(gus.Search)
		//返回所有的用户组, 在本地进行处理组成员关系映射
		groupFilter := filterToLdapSearchFilter(gus.Search.Filter, gus.Search.MemberAttr)
		groupEntries, err := SearchEntriesWithPaging(ctx, config, gus.Search.BaseDN, groupAttributes, groupFilter, dialTimeout)
		if err == nil {
			processed = true
			for _, ue := range userEntries {
				account := entryToAccount(userSearch, *ue)
				if account.Name == "" {
					log.Debugf("LDAP syncAccount, ignore account %v with empty username.\n", account)
					continue
				}
				//找到用户存在用户组成员列表属性中的属性的值
				useMatchAttrVal := getAttr(*ue, gus.Matcher.UserAttr)
				//ignore if user don't has matching attr
				if useMatchAttrVal != "" {
					for _, ge := range groupEntries {
						// 用户组名
						gName := getAttr(*ge, gus.Search.NameAttr)
						if gName == "" {
							continue
						}
						//用户组成员列表属性包含用户的指定属性的值
						gMembers := sets.NewString(getAttrs(*ge, gus.Matcher.GroupAttr)...)
						if gMembers.Has(useMatchAttrVal) {
							account.Group = append(account.Group, gName)
						}
					}
				}
				accounts = append(accounts, account)
			}
		} else {
			log.Infof("ldap: search group with paging fail:%v", err.Error())
		}
	}

	if !processed {
		for _, entry := range userEntries {
			account := entryToAccount(userSearch, *entry)
			if account.Name == "" {
				log.Debugf("LDAP syncAccount, ignore account %v with empty username.\n", account)
				continue
			}

			accounts = append(accounts, account)
		}
	}
	log.Infof("LDAP %+v syncAccount, %v Entry has synced, %v valid Accounts\n", config, len(userEntries), len(accounts))

	return accounts, userEntries, nil
}

func SyncGroups(ctx context.Context, config Config, gs GroupSearch, ugs *UserGroupSearch, dialTimeout time.Duration) ([]Group, error) {
	if gs.Filter == "" {
		return nil, errors.Errorf("missing  search filter")
	}

	if gs.NameAttr == "" {
		return nil, errors.Errorf("missing user name attribute")
	}

	attributes := groupSearchAttributes(gs)
	filter := filterToLdapSearchFilter(gs.Filter)
	entries, err := SearchEntriesWithPaging(ctx, config, gs.BaseDN, attributes, filter, dialTimeout)
	if err != nil {
		return nil, errors.Errorf("ldap: search with paging fail:%v", err.Error())
	}

	groups := make([]Group, 0)
	for _, entry := range entries {
		g := entryToGroup(gs, *entry)
		if g.Name == "" {
			log.Debugf("LDAP syncGroup, ignore account %v with empty name.", g.DN)
			continue
		}

		groups = append(groups, g)
	}
	log.Infof("LDAP %+v syncGroup %v Entry has synced, %v valid Groups\n", config, len(entries), len(groups))
	//要求返回用户信息
	if ugs != nil {
		//确保返回member属性
		gs.MemberAttr = ugs.Matcher.GroupAttr
		userAttributes := userSearchAttributes(ugs.Search)
		userFilter := filterToLdapSearchFilter(ugs.Search.Filter, ugs.Matcher.UserAttr)
		entries, err := SearchEntriesWithPaging(ctx, config, ugs.Search.BaseDN, userAttributes, userFilter, dialTimeout)
		if err != nil {
			return nil, errors.Errorf("ldap: search with paging fail:%v", err.Error())
		}

		for i := range groups {
			memberset := sets.NewString(groups[i].members...)
			for _, entry := range entries {
				if memberset.Has(getAttr(*entry, ugs.Matcher.UserAttr)) {
					groups[i].Members = append(groups[i].Members, getAttr(*entry, ugs.Search.UserNameAttr))
				} else {
					log.Debugf("ignore entry %v doesn't contain matcher user attr %v", entry.DN, ugs.Search.UserNameAttr)
				}
			}
			groups[i].members = nil
		}
	}
	return groups, nil
}

// 同时同步用户组和用户信息
func SyncGroupsAccounts(ctx context.Context, config Config, us *UserSearch, gs *GroupSearch, matcher *UserGroupMatcher, dialTimeout time.Duration) ([]Group, []Account, error) {
	if us == nil && gs == nil {
		return nil, nil, errors.Errorf("UserSearch GroupSearch must set at least one")
	}
	if matcher != nil {
		if matcher.UserAttr == "" || matcher.GroupAttr == "" {
			return nil, nil, errors.Errorf("UserAttr and GroupAttr must set when matcher is set")
		}

		if gs != nil && gs.MemberAttr != matcher.GroupAttr {
			return nil, nil, errors.Errorf("groupSearch.MemberAttr not equal to matcher.GroupAttr")
		}
	}

	var err error
	var userEntries []*ldap.Entry
	if us != nil {
		userAttributes := userSearchAttributes(*us)
		userFilter := filterToLdapSearchFilter(us.Filter)
		if matcher != nil {
			userFilter = filterToLdapSearchFilter(us.Filter, matcher.UserAttr)
		}

		userEntries, err = SearchEntriesWithPaging(ctx, config, us.BaseDN, userAttributes, userFilter, dialTimeout)
		if err != nil {
			return nil, nil, errors.Errorf("ldap: search user %v with paging fail:%v", us, err.Error())
		}
		log.Infof("LDAP %+v syncUser %v Entry has synced", config, len(userEntries))
	}

	var groupEntries []*ldap.Entry
	if gs != nil {
		if gs.Filter == "" {
			return nil, nil, errors.Errorf("missing group search filter")
		}

		if gs.NameAttr == "" {
			return nil, nil, errors.Errorf("missing group name attribute")
		}

		attributes := groupSearchAttributes(*gs)
		groupFilter := filterToLdapSearchFilter(gs.Filter)
		if matcher != nil {
			groupFilter = filterToLdapSearchFilter(gs.Filter, matcher.GroupAttr)
		}
		groupEntries, err = SearchEntriesWithPaging(ctx, config, gs.BaseDN, attributes, groupFilter, dialTimeout)
		if err != nil {
			return nil, nil, errors.Errorf("ldap: search group %v with paging fail:%v", gs, err.Error())
		}
		log.Infof("LDAP %+v syncGroup %v Entry has synced", config, len(groupEntries))
	}

	var groups []Group
	if groupEntries != nil {
		for _, entry := range groupEntries {
			g := entryToGroup(*gs, *entry)
			if g.Name == "" {
				log.Debugf("LDAP syncGroup, ignore account %v with empty name.", g.DN)
				continue
			}

			groups = append(groups, g)
		}
		log.Infof("LDAP %+v syncGroup %v Entry has synced, %v valid Groups\n", config, len(groupEntries), len(groups))

		if matcher != nil {
			for i := range groups {
				memberset := sets.NewString(groups[i].members...)
				log.Debugf("group %v  member size:%v", groups[i].DN, len(groups[i].members))
				for _, ue := range userEntries {
					if memberset.Has(getAttr(*ue, matcher.UserAttr)) {
						groups[i].Members = append(groups[i].Members, getAttr(*ue, us.UserNameAttr))
					} else {
						log.Debugf("ignore entry %v doesn't contain matcher user attr %v", ue.DN, us.UserNameAttr)
					}
				}
				groups[i].members = nil
			}
		}
	}
	//要求返回用户信息
	var accounts []Account
	if userEntries != nil {
		for _, ue := range userEntries {
			account := entryToAccount(*us, *ue)
			if account.Name == "" {
				log.Debugf("LDAP syncAccount, ignore account %v with empty username.\n", account)
				continue
			}
			//找到用户存在用户组成员列表属性中的属性的值
			if matcher != nil {
				useMatchAttrVal := getAttr(*ue, matcher.UserAttr)
				//ignore if user don't has matching attr
				if useMatchAttrVal != "" {
					for _, ge := range groupEntries {
						// 用户组名
						gName := getAttr(*ge, gs.NameAttr)
						if gName == "" {
							continue
						}
						//用户组成员列表属性包含用户的指定属性的值
						gMembers := sets.NewString(getAttrs(*ge, matcher.GroupAttr)...)
						if gMembers.Has(useMatchAttrVal) {
							account.Group = append(account.Group, gName)
						}
					}
				}
			}
			accounts = append(accounts, account)
		}
	}
	return groups, accounts, nil
}

// Authentication 通过ldap用户验证ldap用户权限
// 验证分为两个流程：1. 通过搜索得到用户的信息, 2.使用DN去验证密码
func Authentication(ctx context.Context, config Config, userSearch UserSearch, userAttr, userAttrValue, password string, gus *GroupUserSearch, dialTimeout /*连接超时*/ time.Duration) (*Account, error) {
	userSearch.Filter = fmt.Sprintf("(%s=%s)", userAttr, userAttrValue)
	accounts, userEntries, err := SyncAccounts(ctx, config, userSearch, nil, dialTimeout)
	if err != nil {
		return nil, err
	}

	if len(accounts) != 1 {
		return nil, errors.Errorf("User does not exist or too many with filter %v entries(%v) returned.", userSearch.Filter, len(accounts))
	}

	if err := _do(config, dialTimeout, func(c *ldap.Conn) error {
		if err := c.Bind(accounts[0].DN, password); err != nil {
			if strings.Contains(err.Error(), "Invalid Credentials") {
				return errAuthenticateFail
			}
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// find login account user groups
	if gus != nil {
		gus.Search.MemberAttr = gus.Matcher.GroupAttr
		groupAttributes := groupSearchAttributes(gus.Search)
		//搜索member中包含指定用户属性的
		groupFilter := filterToLdapSearchFilter(gus.Search.Filter, fmt.Sprintf("%v=%v", gus.Search.MemberAttr, getAttr(*userEntries[0], gus.Matcher.UserAttr)))
		groupEntries, err := SearchEntriesWithPaging(ctx, config, gus.Search.BaseDN, groupAttributes, groupFilter, dialTimeout)
		if err == nil {
			for _, ge := range groupEntries {
				gName := getAttr(*ge, gus.Search.NameAttr)
				if gName != "" {
					accounts[0].Group = append(accounts[0].Group, gName)
				}
			}
		} else {
			log.Infof("ignore ldap group append for searching fail:%v", err.Error())
		}
	}

	return &accounts[0], nil
}

// core search function
func SearchEntriesWithPaging(ctx context.Context, config Config, baseDN string, attributes []string, filter string, dialTimeout time.Duration) ([]*ldap.Entry, error) {
	entries := make([]*ldap.Entry, 0)
	if err := _do(config, dialTimeout, func(c *ldap.Conn) error {
		var err error
		entries, err = _searchEntriesWithPaging(c, baseDN, attributes, filter)
		return err
	}); err != nil {
		log.Infof("SearchEntriesWithPaging fail:%v", err.Error())
		return nil, err
	}
	return entries, nil
}

// search everything in ldap according to filter. so remember pass right filter to get which kind object you want.
// filter:
//  1. 过滤搜索到的元素。可以将某个attributes属性作为filter项来精确查找 。如：用户组类filter为(objectClass=group),用户组中的通过member属性记录用户名.
//  2. 如果需要查找某个用户user1所有的用户组,则可以在(object=group)(member=user1)来找用户所有用户组
//     补充: 即使member是一个数组,也能正常查询
//  3. dn作为过滤的条件时,无论是(dn=*),还是(dn=cn=jane,ou=People,dc=example,dc=org)返回的entries都为空。

// attributes:
//  1. 返回的元素携带哪些属性. 如果attributes==nil,返回全部属性(具体属性由LDAP服务器决定)。
//  2. 注意attribute和filter是独立的。先通过filter过滤出元素,再根据attribute决定哪些属性通过entry携带返回客户端
func _searchEntriesWithPaging(conn *ldap.Conn, baseDN string, attributes []string, filter string) ([]*ldap.Entry, error) {
	if baseDN == "" {
		return nil, errors.Errorf("baseDN is empty")
	}

	if !strings.HasPrefix(filter, "(") || !strings.HasSuffix(filter, ")") {
		return nil, errors.Errorf("filter `%v`is not start with `(` or end with `)`", filter)
	}

	// ldap server has size limit, if exceed limit ,it will return Size Exceed Limit
	// most ad server default limit is 1000.
	// so we use 500 as paging size
	pagingControl := ldap.NewControlPaging(500)
	controls := []ldap.Control{pagingControl}
	entries := make([]*ldap.Entry, 0, 500)

	log.Infof("search ldap paging with base dn: %v, attributes:%v, filter:%v", baseDN, attributes, filter)
	for {
		searchRequest := ldap.NewSearchRequest(
			baseDN,
			ldap.ScopeWholeSubtree,
			ldap.NeverDerefAliases,
			0,
			// What is timeLimit meaning?
			// if set timelimit=int(30*time.Second) , it work well in windows AD ,but  openLDAP sever logs `op=1 DISCONNECT tag=120 err=2 text=decoding error`.
			//  and ldap client got` unable to read LDAP response packet: unexpected EOF` error
			// set timelimit = 30, work well in openLDAP.
			0,
			false,
			filter,     // (&(cn=*)(objectClass=person)) or (&(objectClass=person)(mail=janedoe@example.com))"
			attributes, // if attributes == nil, entries returned carry all its attributes;attributes != nil, entries will only carry attributes in `attribute`.
			controls,   // control paging
		)

		sr, err := conn.Search(searchRequest)
		if err != nil {
			return nil, errors.Errorf("Failed to execute search request: %v", err.Error())
		}
		entries = append(entries, sr.Entries...)
		// In order to prepare the next request, we check if the response
		// contains another ControlPaging object and a not-empty cookie and
		// copy that cookie into our pagingControl object:
		updatedControl := ldap.FindControl(sr.Controls, ldap.ControlTypePaging)
		if ctrl, ok := updatedControl.(*ldap.ControlPaging); ctrl != nil && ok && len(ctrl.Cookie) != 0 {
			pagingControl.SetCookie(ctrl.Cookie)
			continue
		}
		break
	}
	log.Infof("base dn: %v, attributes:%v, filter:%v, %v entries has synced", baseDN, attributes, filter, len(entries))
	return entries, nil
}
