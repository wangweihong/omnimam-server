package example

import (
	"context"
	"fmt"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/ldap"
	"github.com/wangweihong/gotoolbox/pkg/sets"
	"github.com/wangweihong/gotoolbox/pkg/waitgroup"

	"github.com/sirupsen/logrus"
)

// 同步缓存和LDAP的用户组，已删除的清理，新创建的则在缓存中创建
func syncLDAPGroup(ldapGroups []ldap.Group) waitgroup.BatchOutput {
	type Mapper struct {
		Cache *Group
		LDAP  *ldap.Group
	}

	mappers := make(map[string]Mapper)
	for i := range ldapGroups {
		mappers[ldapGroups[i].Name] = Mapper{
			LDAP: &ldapGroups[i],
		}
	}

	cacheGroup := CacheInstance.ListGroups()
	for i := range cacheGroup {
		if cacheGroup[i].IsLDAP {
			if m, exist := mappers[cacheGroup[i].Name]; exist {
				m.Cache = &cacheGroup[i]
				mappers[cacheGroup[i].Name] = m
			} else {
				mappers[cacheGroup[i].Name] = Mapper{
					Cache: &cacheGroup[i],
				}
			}
		}
	}

	wg := waitgroup.NewWaitGroup(context.Background())
	for k, u := range mappers {
		u, k := u, k

		if u.LDAP != nil && u.Cache == nil {
			wg.Start(waitgroup.NewFunc("", func(ctx context.Context) waitgroup.Result {
				tmp := Group{
					Name:   k,
					IsLDAP: true,
				}
				if _, err := CacheInstance.CreateGroup(&tmp); err != nil {
					return waitgroup.NewResult("ldap group add:"+k, err)
				}
				return waitgroup.NewResult("ldap group add:"+k, nil)
			}))
		}

		if u.Cache != nil && u.LDAP == nil {
			wg.Start(waitgroup.NewFunc("", func(ctx context.Context) waitgroup.Result {
				if err := CacheInstance.DeleteGroup(u.Cache.UUID); err != nil {
					return waitgroup.NewResult("ldap group delete:"+k, err)
				}
				return waitgroup.NewResult("ldap group delete:"+k, nil)
			}))
		}
	}
	wg.Wait()
	return wg.ConvertResultToBatchOutput()
}

// 同步缓存和LDAP的用户，已删除的清理，新创建的则在缓存中创建. 根据ldap跳转缓存用户分组
func syncLdapUser(ldapUsers []ldap.Account) waitgroup.BatchOutput {
	//用来记录用户在缓存和LDAP的映射
	// Cache != nil && LDAP != nil : 用户同时存在缓存和LDAP
	// Cache != nil && LDAP == nil : 用户只存在本地
	// Cache == nil && LDAP != nil : 用户只存在LDAP
	type Mapper struct {
		Cache *User
		LDAP  *ldap.Account
	}

	mappers := make(map[string]Mapper)
	for i := range ldapUsers {
		mappers[ldapUsers[i].Name] = Mapper{
			LDAP: &ldapUsers[i],
		}
	}
	logrus.Debugf("mapper nums:%v", len(mappers))
	// calculate exist ldap users
	cacheUsers := CacheInstance.ListUsers()
	for i := range cacheUsers {
		if cacheUsers[i].IsLDAP {
			if m, exist := mappers[cacheUsers[i].Name]; exist {
				m.Cache = &cacheUsers[i]
				mappers[cacheUsers[i].Name] = m
			} else {
				mappers[cacheUsers[i].Name] = Mapper{
					Cache: &cacheUsers[i],
				}
			}
		}
	}

	cacheGroup := CacheInstance.ListGroups()
	// name -> group
	existCacheLDAPGroupSet := make(map[string]*Group)
	for i := range cacheGroup {
		if cacheGroup[i].IsLDAP {
			existCacheLDAPGroupSet[cacheGroup[i].Name] = &cacheGroup[i]
		}
	}

	ctx := context.Background()
	wg := waitgroup.NewWaitGroup(ctx)
	for k, u := range mappers {
		k, u := k, u
		// user exist in cache and ldap both, update if group has changed
		if u.Cache != nil && u.LDAP != nil {
			newGroupIDS := sets.NewString()
			changed := true
			if len(u.LDAP.Group) != 0 {
				for _, eg := range u.LDAP.Group {
					//记录LDAP用户中用户组名对应的组ID
					if g, exist := existCacheLDAPGroupSet[eg]; exist {
						newGroupIDS.Insert(g.UUID)
					}
				}
				// 比对缓存中的用户组ID和ldap用户组ID
				if newGroupIDS.Equal(sets.NewString(u.Cache.UserGroup...)) {
					changed = false
				}
			}
			// LDAP用户组已经更新, 本地进行同步
			if changed {
				wg.Start(waitgroup.NewFunc("", func(ctx context.Context) waitgroup.Result {
					u.Cache.UserGroup = newGroupIDS.List()
					if err := CacheInstance.UpdateUser(u.Cache); err != nil {
						return waitgroup.NewResult("ldap update user:"+k, err)
					}
					return waitgroup.NewResult("ldap update user:"+k, nil)
				}))
			}

			continue
		}
		//ldap has user not exist in cache. need to create.
		if u.LDAP != nil {
			wg.Start(waitgroup.NewFunc("", func(ctx context.Context) waitgroup.Result {
				var groupIDs []string
				newGroupIDS := sets.NewString()
				for _, eg := range u.LDAP.Group {
					//find ldap group uuid
					if g, exist := existCacheLDAPGroupSet[eg]; exist {
						newGroupIDS.Insert(g.UUID)
					}
				}

				if newGroupIDS.Len() != 0 {
					groupIDs = newGroupIDS.List()
				}

				tmp := User{
					Name:       u.LDAP.Name,
					CreateTime: u.LDAP.CreateTime,
					Email:      &u.LDAP.Email,
					Phone:      &u.LDAP.Phone,
					UserGroup:  groupIDs,
					IsLDAP:     true,
				}
				if _, err := CacheInstance.CreateUser(&tmp); err != nil {
					return waitgroup.NewResult("ldap new user:"+k, err)
				}
				return waitgroup.NewResult("ldap new user:"+k, nil)
			}))
			continue
		}
		// cache user not exist in ldap
		if u.Cache != nil {
			wg.Start(waitgroup.NewFunc("", func(ctx context.Context) waitgroup.Result {
				if err := CacheInstance.DeleteUser(u.Cache.UUID); err != nil {
					return waitgroup.NewResult("ldap Delete user:"+k, err)
				}
				return waitgroup.NewResult("ldap Delete user:"+k, nil)
			}))
			continue
		}
	}
	wg.Wait()

	return wg.ConvertResultToBatchOutput()
}

func SyncLdapUser(ctx context.Context, config *ldap.Config, us ldap.UserSearch, gs *ldap.GroupSearch, matcher *ldap.UserGroupMatcher) (waitgroup.BatchOutput, error) {
	groupResult := waitgroup.BatchOutput{}
	ldapGroups, ldapUsers, err := ldap.SyncGroupsAccounts(
		ctx,
		*config,
		&us,
		gs,
		matcher,
		60*time.Second,
	)
	if err != nil {
		return groupResult, fmt.Errorf("sync ldap group accounts error:%v", err)
	}

	if ldapGroups != nil {
		groupResult = syncLDAPGroup(ldapGroups)
	}

	userResult := syncLdapUser(ldapUsers)
	userResult.Merge(&groupResult)

	return userResult, nil
}
