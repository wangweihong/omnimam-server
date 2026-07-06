package example

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/json"
)

var mc = &mockCache{
	lock:   sync.Mutex{},
	users:  make(map[string]User),
	groups: make(map[string]Group),
}
var CacheInstance CacheInterface = mc

type User struct {
	Name       string    `json:"name"`
	UUID       string    `json:"uuid"`
	CreateTime time.Time `json:"create_time"`
	ModifyTime time.Time `json:"modify_time"`
	Password   string    `json:"password"`
	Desc       *string   `json:"desc"`
	Email      *string   `json:"email"`
	Phone      *string   `json:"phone"`
	UserGroup  []string  // 存放user group uuid
	IsLDAP     bool      `json:"is_ldap"`
}

func (u *User) DeepCopy() *User {
	var d User
	deepCopy(u, &d)
	return &d
}

type Group struct {
	Name       string    `json:"name"`
	UUID       string    `json:"uuid"`
	CreateTime time.Time `json:"create_time"`
	ModifyTime time.Time `json:"modify_time"`
	IsLDAP     bool      `json:"is_ldap"`
}

func (g *Group) DeepCopy() *Group {
	var d Group
	deepCopy(g, &d)
	return &d
}

type CacheInterface interface {
	Print()
	Clean()
	ListUsers() []User
	DeleteUser(UUID string) error
	CreateUser(u *User) (*User, error)
	UpdateUser(u *User) error

	ListGroups() []Group
	CreateGroup(g *Group) (*Group, error)
	DeleteGroup(UUID string) error
}

type mockCache struct {
	lock   sync.Mutex
	users  map[string]User
	groups map[string]Group
}

func (m *mockCache) Print() {
	m.lock.Lock()
	defer m.lock.Unlock()

	json.PrintStructObject(m.users)
	json.PrintStructObject(m.groups)
}

func (m *mockCache) Clean() {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.users = make(map[string]User)
	m.groups = make(map[string]Group)
}

func (m *mockCache) ListUsers() []User {
	m.lock.Lock()
	defer m.lock.Unlock()

	us := make([]User, 0, len(m.users))
	for _, v := range m.users {
		us = append(us, *v.DeepCopy())
	}
	return us
}

func (m *mockCache) DeleteUser(UUID string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	delete(m.users, UUID)
	return nil
}

func (m *mockCache) CreateUser(u *User) (*User, error) {
	if u.Name == "" {
		return nil, fmt.Errorf("user name is empty")
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, v := range m.users {
		if v.Name == u.Name && u.IsLDAP == v.IsLDAP {
			return nil, fmt.Errorf("user name %v ldap:%v has exist", u.Name, u.IsLDAP)
		}
	}
	meta := u.DeepCopy()

	meta.UUID = uuid.New().String()
	meta.CreateTime = time.Now()
	meta.ModifyTime = meta.CreateTime

	m.users[meta.UUID] = *meta
	return u.DeepCopy(), nil
}

func (m *mockCache) UpdateUser(u *User) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	originMeta, ok := m.users[u.UUID]
	if !ok {
		return fmt.Errorf("user %v not exist", u.UUID)
	}
	meta := originMeta.DeepCopy()

	meta.ModifyTime = time.Now()
	if u.UserGroup != nil {
		meta.UserGroup = u.UserGroup
	}

	if u.Phone != nil {
		meta.Phone = u.Phone
	}

	if u.Email != nil {
		meta.Email = u.Email
	}

	if u.Desc != nil {
		meta.Desc = u.Desc
	}

	if u.Password != "" {
		meta.Password = u.Password
	}

	m.users[meta.UUID] = *meta
	return nil
}

func (m *mockCache) ListGroups() []Group {
	m.lock.Lock()
	defer m.lock.Unlock()

	us := make([]Group, 0, len(m.groups))
	for _, v := range m.groups {
		us = append(us, *v.DeepCopy())
	}
	return us
}

func (m *mockCache) CreateGroup(g *Group) (*Group, error) {
	if g.Name == "" {
		return nil, fmt.Errorf("name is empty")
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, v := range m.users {
		if v.Name == g.Name && v.IsLDAP == g.IsLDAP {
			return nil, fmt.Errorf("name has exist")
		}
	}
	meta := g.DeepCopy()
	meta.UUID = uuid.New().String()
	meta.CreateTime = time.Now()
	meta.ModifyTime = meta.CreateTime

	m.groups[meta.UUID] = *meta

	return meta.DeepCopy(), nil
}

func (m *mockCache) DeleteGroup(UUID string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	delete(m.groups, UUID)
	return nil
}

func deepCopy(a, b interface{}) error {
	byt, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("deep copy fail:%v", err.Error())
	}
	err = json.Unmarshal(byt, b)
	if err != nil {
		return fmt.Errorf("deep copy fail:%v", err.Error())
	}

	return nil
}
