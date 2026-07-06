package cache

import (
	"fmt"
	"sync"

	"github.com/wangweihong/gotoolbox/pkg/sets"
)

// ThreadSafeStore is an interface that allows concurrent indexed
// access to a storage backend.  It is like Indexer but does not
// (necessarily) know how to extract the Store key from a given
// object.
//
// TL;DR caveats: you must not modify anything returned by Get or List as it will break
// the indexing feature in addition to not being thread safe.
//
// The guarantees of thread safety provided by List/Get are only valid if the caller
// treats returned items as read-only. For example, a pointer inserted in the store
// through `Add` will be returned as is by `Get`. Multiple clients might invoke `Get`
// on the same key and modify the pointer in a non-thread-safe way. Also note that
// modifying objects stored by the indexers (if any) will *not* automatically lead
// to a re-index. So it's not a good idea to directly modify the objects returned by
// Get/List, in general.
type ThreadSafeStore interface {
	Add(key string, obj interface{})          // 替换对象，以指定key作为索引
	Inject(key string, obj interface{}) error // 增加对象，以指定key作为索引
	Update(key string, obj interface{}) error
	Delete(key string)
	Get(key string) (item interface{}, exists bool)
	List() []interface{}
	ListKeys() []string
	Replace(map[string]interface{}, string)
	// 如索引器为租户, object的租户为tenant1, 则返回所有tenant1的对象列表
	Index(indexName string, obj interface{}) ([]interface{}, error)
	IndexKeys(indexName, indexKey string) ([]string, error)
	ListIndexFuncValues(name string) []string
	ByIndex(indexName, indexKey string) ([]interface{}, error)
	GetIndexers() Indexers

	// AddIndexers adds more indexers to this store.  If you call this after you already have data
	// in the store, the results are undefined.
	AddIndexers(newIndexers Indexers) error
	// Resync is a no-op and is deprecated
	Resync() error
}

// 提供一个支持快速索引的本地缓存表.
type threadSafeMap struct {
	lock  sync.RWMutex
	items map[string]interface{} // 真正存储对象的表

	// indexers maps a name to an IndexFunc
	indexers Indexers // 索引器列表.key为索引器，值为如果通过对象计算索引
	// 本质为 map[索引器名][索引函数]
	// indices maps a name to an Index
	indices Indices //  存储每个索引器对应索引和索引值表。 注意最后存储的索引值是索引的对象的键，而不是索引的对象。找到索引值后，再去items中取对象
	//  本质为map[索引器名]map[索引1]map[值1]
	//								[值2]
}

// 添加指定对象,并基于索引器建立索引.
func (c *threadSafeMap) Add(key string, obj interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	oldObject := c.items[key]
	c.items[key] = obj
	c.updateIndices(oldObject, obj, key)
}

func (c *threadSafeMap) Inject(key string, obj interface{}) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	oldObject := c.items[key]
	if oldObject != nil {
		return fmt.Errorf("object %v exist", key)
	}

	c.items[key] = obj
	c.updateIndices(nil, obj, key)
	return nil
}

// 添加某个对象，并基于索引器建立索引.
func (c *threadSafeMap) Update(key string, obj interface{}) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	oldObject := c.items[key]
	if oldObject == nil {
		return fmt.Errorf("object %v not exist", key)
	}

	c.items[key] = obj
	c.updateIndices(oldObject, obj, key)
	return nil
}

// 删除某个对象，并将该对象从索引表中移除.
func (c *threadSafeMap) Delete(key string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if obj, exists := c.items[key]; exists {
		c.deleteFromIndices(obj, key)
		delete(c.items, key)
	}
}

// Get 返回指定的对象.
func (c *threadSafeMap) Get(key string) (item interface{}, exists bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	item, exists = c.items[key]
	return item, exists
}

// List 返回所有的对象.
func (c *threadSafeMap) List() []interface{} {
	c.lock.RLock()
	defer c.lock.RUnlock()
	list := make([]interface{}, 0, len(c.items))
	for _, item := range c.items {
		list = append(list, item)
	}
	return list
}

// ListKeys returns a list of all the keys of the objects currently
// in the threadSafeMap.
// ListKeys  返回所有的对象的ID.
func (c *threadSafeMap) ListKeys() []string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	list := make([]string, 0, len(c.items))
	for key := range c.items {
		list = append(list, key)
	}
	return list
}

// 更新缓存中的对象表,并且更新对应的索引器.
func (c *threadSafeMap) Replace(items map[string]interface{}, resourceVersion string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.items = items

	// rebuild any index
	c.indices = Indices{}
	for key, item := range c.items {
		c.updateIndices(nil, item, key)
	}
}

// Index returns a list of items that match the given object on the index function.
// Index is thread-safe so long as you treat all items as immutable.
// 如索引器为租户, object的租户为tenant1, 则返回所有tenant1的对象列表.
func (c *threadSafeMap) Index(indexName string, obj interface{}) ([]interface{}, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	indexFunc := c.indexers[indexName]
	if indexFunc == nil {
		return nil, fmt.Errorf("Index with name %s does not exist", indexName)
	}

	indexedValues, err := indexFunc(obj)
	if err != nil {
		return nil, err
	}
	index := c.indices[indexName]

	var storeKeySet sets.String
	if len(indexedValues) == 1 {
		// In majority of cases, there is exactly one value matching.
		// Optimize the most common path - deduping is not needed here.
		storeKeySet = index[indexedValues[0]]
	} else {
		// Need to de-dupe the return list.
		// Since multiple keys are allowed, this can happen.
		storeKeySet = sets.String{}
		for _, indexedValue := range indexedValues {
			for key := range index[indexedValue] {
				storeKeySet.Insert(key)
			}
		}
	}

	list := make([]interface{}, 0, storeKeySet.Len())
	for storeKey := range storeKeySet {
		list = append(list, c.items[storeKey])
	}
	return list, nil
}

// ByIndex returns a list of the items whose indexed values in the given index include the given indexed value
// 如索引器为租户，索引值为tenant1, 返回tenant1相关所有对象列表.
func (c *threadSafeMap) ByIndex(indexName, indexedValue string) ([]interface{}, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	// 找到指定的索引器
	indexFunc := c.indexers[indexName]
	if indexFunc == nil {
		return nil, fmt.Errorf("Index with name %s does not exist", indexName)
	}
	// 索引值表
	index := c.indices[indexName]
	// 指定值的对象ID集合
	set := index[indexedValue]
	list := make([]interface{}, 0, set.Len())
	for key := range set {
		list = append(list, c.items[key])
	}

	return list, nil
}

// IndexKeys returns a list of the Store keys of the objects whose indexed values in the given index include the given
// indexed value.
// IndexKeys is thread-safe so long as you treat all items as immutable.
// 如索引器为租户，索引值为tenant1, 返回tenant1相关所有对象的ID.
func (c *threadSafeMap) IndexKeys(indexName, indexedValue string) ([]string, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	indexFunc := c.indexers[indexName]
	if indexFunc == nil {
		return nil, fmt.Errorf("Index with name %s does not exist", indexName)
	}

	index := c.indices[indexName]

	set := index[indexedValue]
	return set.List(), nil
}

// 返回指定索引器中索引名.
func (c *threadSafeMap) ListIndexFuncValues(indexName string) []string {
	c.lock.RLock()
	defer c.lock.RUnlock()

	index := c.indices[indexName]
	names := make([]string, 0, len(index))
	for key := range index {
		names = append(names, key)
	}
	return names
}

// 提取缓存表之前所有的索引器.
func (c *threadSafeMap) GetIndexers() Indexers {
	return c.indexers
}

// 增加一组新的索引器到缓存表中。 新的索引器如果之前存在则报错.
func (c *threadSafeMap) AddIndexers(newIndexers Indexers) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if len(c.items) > 0 {
		return fmt.Errorf("cannot add indexers to running index")
	}

	// 提取之前的索引器列表
	oldKeys := sets.StringKeySet(c.indexers)
	// 提取新的索引器表
	newKeys := sets.StringKeySet(newIndexers)

	// 如果新的索引器之前已经存在，则报错
	if oldKeys.HasAny(newKeys.List()...) {
		return fmt.Errorf("indexer conflict: %v", oldKeys.Intersection(newKeys))
	}

	for k, v := range newIndexers {
		c.indexers[k] = v
	}
	return nil
}

// updateIndices modifies the objects location in the managed indexes, if this is an update, you must provide an oldObj
// updateIndices must be called from a function that already has a lock on the cache.
func (c *threadSafeMap) updateIndices(oldObj interface{}, newObj interface{}, key string) {
	// if we got an old object, we need to remove it before we add it again
	// 如果之前的对象已经存在,则先将老的索引移除掉
	if oldObj != nil {
		c.deleteFromIndices(oldObj, key)
	}
	// 每个索引器计算出对象新的索引，并插入到对应索引表中
	for name, indexFunc := range c.indexers {
		// 算出新的对象的索引值
		indexValues, err := indexFunc(newObj)
		if err != nil {
			panic(fmt.Errorf("unable to calculate an index entry for key %q on index %q: %w", key, name, err))
		}
		index := c.indices[name]
		if index == nil {
			index = Index{}
			c.indices[name] = index
		}

		for _, indexValue := range indexValues {
			set := index[indexValue]
			if set == nil {
				set = sets.String{}
				index[indexValue] = set
			}
			set.Insert(key)
		}
	}
}

// deleteFromIndices removes the object from each of the managed indexes
// it is intended to be called from a function that already has a lock on the cache
// 把指定对象从所有索引的表中移除.
func (c *threadSafeMap) deleteFromIndices(obj interface{}, key string) {
	// 遍历全部索引器
	for name, indexFunc := range c.indexers {
		// 通过索引器函数计算出对象相关联索引的一系列相关值。
		// 例如索引器为命名空间索引器, 索引器函数则为提取指定对象的命名空间，并返回记录在索引器表中该命名空间下的所有索引值。
		// 索引值具体是什么，uuid/id/name取决于添加到索引器的设计。
		indexValues, err := indexFunc(obj)
		if err != nil {
			panic(fmt.Errorf("unable to calculate an index entry for key %q on index %q: %w", key, name, err))
		}
		// 如果索引器已经被删除，则忽略
		index := c.indices[name]
		if index == nil {
			continue
		}
		//  删除对象在索引器表中对应的索引
		for _, indexValue := range indexValues {
			set := index[indexValue]
			if set != nil {
				set.Delete(key)

				// 如果索引表值已经空了，就清掉
				if len(set) == 0 {
					delete(index, indexValue)
				}
			}
		}
	}
}

func (c *threadSafeMap) Resync() error {
	// Nothing to do
	return nil
}

// NewThreadSafeStore creates a new instance of ThreadSafeStore.
func NewThreadSafeStore(indexers Indexers, indices Indices) ThreadSafeStore {
	return &threadSafeMap{
		items:    map[string]interface{}{},
		indexers: indexers,
		indices:  indices,
	}
}
