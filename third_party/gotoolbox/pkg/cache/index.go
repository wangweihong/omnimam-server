package cache

import "github.com/wangweihong/gotoolbox/pkg/sets"

type Indexer interface {
	Store
	// Index returns the stored objects whose set of indexed values
	// intersects the set of indexed values of the given object, for
	// the named index
	// 找到指定对象在某个索引器中的索引值相关联的一系列对象。
	// 如索引器为租户, object的租户为tenant1,tenant2, 则返回所有tenant1、tenant2的对象列表
	Index(indexName string, obj interface{}) ([]interface{}, error)
	// IndexKeys returns the storage keys of the stored objects whose
	// set of indexed values for the named index includes the given
	// indexed value
	// 如索引器为租户, object的租户为tenant1, 则返回所有tenant1的对象ID列表
	IndexKeys(indexName, indexedValue string) ([]string, error)
	// ListIndexFuncValues returns all the indexed values of the given index
	ListIndexFuncValues(indexName string) []string
	// ByIndex returns the stored objects whose set of indexed values
	// for the named index includes the given indexed value
	// 如索引器为租户, 则返回租户indexedValue下的所有用户
	ByIndex(indexName, indexedValue string) ([]interface{}, error)
	// GetIndexer return the indexers
	GetIndexers() Indexers

	// AddIndexers adds more indexers to this store.  If you call this after you already have data
	// in the store, the results are undefined.
	AddIndexers(newIndexers Indexers) error
}

// Index maps the indexed value to a set of keys in the store that match on that value
// 索引器的作用，用于快速定位一类对象的。如在某个缓存表中以UUID作为对象的索引，如果需要查找某个命名空间的对象,只能通过遍历所有
// 对象来比对命名空间。但可以通过索引器，建立一个命名空间:UUID的索引，只需要找到对应的命名空间索引，就能会找到该命名空间
// 下的一系列对象的UUID，再从UUID索引提取对象的具体信息。
// key是通过索引器函数计算出来的。如是命名空间索引器，则key为某个命名空间的名字。
// value是一个set,存储的是某个对象的UUID/ID之类的值。当索引出该值后，就可以从对象表找到某个对象。
type Index map[string]sets.String

// Indexers maps a name to a IndexFunc
// 索引器列表
// key为索引器名，IndexFunc则为如何通过某个对象来计算出该对象的索引值(一个或多个).
type Indexers map[string]IndexFunc

// Indices maps a name to an Index.
type Indices map[string]Index // 索引表。 key为通常存储的是索引器名。

type IndexFunc func(obj interface{}) ([]string, error)
