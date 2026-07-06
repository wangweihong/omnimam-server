package example

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/cache"
	"github.com/wangweihong/gotoolbox/pkg/sets"

	"github.com/google/uuid"
)

type UserManagerInterface interface {
	Add(obj *User) (*User, error)
	Update(obj *User) error
	Delete(obj *User) error
	List() []*User
	ListKeys() []string
	Get(obj interface{}) (item *User, exists bool, err error)
	GetByKey(key string) (item *User, exists bool, err error)
	Replace([]*User, string) error

	ListInTenant(tenant string) []*User
	ListInTenantIndex(tenant string) []string
	ListInGroup(group string) []*User
	ListInGroupIndex(group string) []string
	// 删除用户中某个组的索引
	CleanGroup(group string) error
	ListInRole(role string) []*User
	ListInRoleIndex(role string) []string
	CleanRole(role string) error
}

func (u userManager) Add(obj *User) (*User, error) {
	if obj.Name == "" {
		return nil, fmt.Errorf("user name is empty")
	}

	meta := obj.DeepCopy()
	meta.CreateTime = time.Now()
	meta.UpdateTime = time.Now()
	meta.UUID = uuid.New().String()

	err := u.Indexer.Add(meta)
	if err != nil {
		return nil, err
	}
	return meta.DeepCopy(), err
}

func (u userManager) Update(obj *User) error {
	if obj.UUID == "" {
		return fmt.Errorf("user uuid is empty")
	}
	// TODO: check user data if valid
	meta := obj.DeepCopy()
	meta.UpdateTime = time.Now()

	return u.Indexer.Update(obj)
}

func (u userManager) Delete(obj *User) error {
	return u.Indexer.Delete(obj)
}

func (u userManager) List() []*User {
	objects := u.Indexer.List()
	users := make([]*User, 0, len(objects))
	for _, v := range objects {
		if u, ok := v.(*User); ok {
			users = append(users, u.DeepCopy())
		}
	}
	return users
}

func (u userManager) Get(obj interface{}) (*User, bool, error) {
	meta, exists, err := u.Indexer.Get(obj)
	if err != nil {
		return nil, false, err
	}

	if !exists {
		return nil, false, nil
	}

	item, ok := meta.(*User)
	if !ok {
		return nil, false, errors.New("object is not user")
	}

	return item.DeepCopy(), true, nil
}

func (u userManager) GetByKey(key string) (*User, bool, error) {
	obj, exists, err := u.Indexer.GetByKey(key)
	if err != nil {
		return nil, exists, err
	}

	if !exists {
		return nil, false, nil
	}

	item, ok := obj.(*User)
	if !ok {
		return nil, true, errors.New("object is not User")
	}

	return item, false, nil
}

func (u userManager) Replace(users []*User, s string) error {
	items := make([]interface{}, 0, len(users))
	for _, v := range users {
		items = append(items, v.DeepCopy())
	}

	return u.Indexer.Replace(items, s)
}

func (u userManager) ListInTenant(tenant string) []*User {
	users := make([]*User, 0)

	objects, err := u.Indexer.Index(indexTypeTenantUser, &User{Tenant: tenant})
	if err != nil {
		return users
	}
	for _, v := range objects {
		if u, ok := v.(*User); ok {
			users = append(users, u.DeepCopy())
		}
	}
	return users
}

func (u userManager) ListInTenantIndex(tenant string) []string {
	objects, err := u.Indexer.IndexKeys(indexTypeTenantUser, tenant)
	if err != nil {
		return nil
	}
	return objects
}

func (u userManager) ListInGroup(groups string) []*User {
	users := make([]*User, 0)

	objects, err := u.Indexer.Index(indexTypeGroupUser, &User{Group: []string{groups}})
	if err != nil {
		return users
	}
	for _, v := range objects {
		if u, ok := v.(*User); ok {
			users = append(users, u.DeepCopy())
		}
	}
	return users
}

func (u userManager) ListInGroupIndex(group string) []string {
	objects, err := u.Indexer.IndexKeys(indexTypeGroupUser, group)
	if err != nil {
		return nil
	}
	return objects
}

func (u userManager) CleanGroup(group string) error {
	objects, err := u.Indexer.Index(indexTypeGroupUser, &User{Group: []string{group}})
	if err != nil {
		return err
	}
	for _, v := range objects {
		if user, ok := v.(*User); ok {
			meta := user.DeepCopy()
			meta.Group = sets.NewString(meta.Group...).Delete(group).List()
			if err := u.Indexer.Update(meta); err != nil {
				return err
			}
		}
	}
	return nil
}

func (u userManager) ListInRole(role string) []*User {
	users := make([]*User, 0)

	objects, err := u.Indexer.Index(indexTypeRoleUser, &User{Roles: []string{role}})
	if err != nil {
		return users
	}
	for _, v := range objects {
		if u, ok := v.(*User); ok {
			users = append(users, u.DeepCopy())
		}
	}
	return users
}

func (u userManager) ListInRoleIndex(role string) []string {
	objects, err := u.Indexer.IndexKeys(indexTypeRoleUser, role)
	if err != nil {
		return nil
	}
	return objects
}

func (u userManager) CleanRole(role string) error {
	objects, err := u.Indexer.Index(indexTypeRoleUser, &User{Roles: []string{role}})
	if err != nil {
		return err
	}
	for _, v := range objects {
		if user, ok := v.(*User); ok {
			meta := user.DeepCopy()
			meta.Roles = sets.NewString(meta.Roles...).Delete(role).List()
			if err := u.Indexer.Update(meta); err != nil {
				return err
			}
		}
	}
	return nil
}

var _ UserManagerInterface = &userManager{}

var (
	umOnce     sync.Once
	umInstance *userManager
)

func GetUMInstance() UserManagerInterface {
	umOnce.Do(func() {
		userIndexers := make(map[string]cache.IndexFunc)
		userIndexers[indexTypeTenantUser] = tenantUserIndexer
		userIndexers[indexTypeGroupUser] = groupUserIndexer
		userIndexers[indexTypeRoleUser] = roleUserIndexer

		umInstance = &userManager{
			Indexer: cache.NewIndexer(userKeyFunc, userIndexers),
		}
	})
	return umInstance
}

func NewUMInstance() UserManagerInterface {
	userIndexers := make(map[string]cache.IndexFunc)
	userIndexers[indexTypeTenantUser] = tenantUserIndexer
	userIndexers[indexTypeGroupUser] = groupUserIndexer
	userIndexers[indexTypeRoleUser] = roleUserIndexer

	return &userManager{
		Indexer: cache.NewIndexer(userKeyFunc, userIndexers),
	}
}

type userManager struct {
	cache.Indexer
}

func userKeyFunc(obj interface{}) (string, error) {
	if obj == nil {
		return "", fmt.Errorf("object is nil")
	}
	user, ok := obj.(*User)
	if !ok {
		return "", fmt.Errorf("object is %v,not %v type", reflect.TypeOf(obj), reflect.TypeOf(&User{}))
	}
	return user.UUID, nil
}

// Object support indexer type.
const (
	indexTypeTenantUser = "tenantUser"
	indexTypeGroupUser  = "groupUser"
	indexTypeRoleUser   = "roleUser"
)

func tenantUserIndexer(obj interface{}) ([]string, error) {
	user, ok := obj.(*User)
	if !ok {
		return []string{""}, fmt.Errorf("object is %v,not %v type", reflect.TypeOf(obj), reflect.TypeOf(&User{}))
	}
	return []string{user.Tenant}, nil
}

func groupUserIndexer(obj interface{}) ([]string, error) {
	user, ok := obj.(*User)
	if !ok {
		return []string{""}, fmt.Errorf("object is %v,not %v type", reflect.TypeOf(obj), reflect.TypeOf(&User{}))
	}
	return user.Group, nil
}

func roleUserIndexer(obj interface{}) ([]string, error) {
	user, ok := obj.(*User)
	if !ok {
		return []string{""}, fmt.Errorf("object is %v,not %v type", reflect.TypeOf(obj), reflect.TypeOf(&User{}))
	}
	return user.Roles, nil
}
