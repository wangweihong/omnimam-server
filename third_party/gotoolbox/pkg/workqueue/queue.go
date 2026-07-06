package workqueue

import (
	"sync"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/clock"
)

type Interface interface {
	// 写入一个数据，唤醒一个等待的工作进程
	Add(item interface{})
	// 队列长度
	Len() int

	Get() (item interface{}, shutdown bool)
	// 某个对象已经被工作线程处理，从正在处理表中移除；如果该对象又被更新了（需要再次处理），再次将对象新数据进行排队，并唤醒一个工作线程开始处理排队的对象
	Done(item interface{})
	// 关闭队列，这会告知所有在等待队列数据的协程
	ShutDown()
	// 检测队列是否正常关闭
	ShuttingDown() bool
}

// New constructs a new work queue.
func New() *Type {
	return NewNamed("")
}

func NewNamed(name string) *Type {
	rc := clock.RealClock{}
	return newQueue(
		rc,
		globalMetricsFactory.newQueueMetrics(name, rc),
		defaultUnfinishedWorkUpdatePeriod,
	)
}

func newQueue(c clock.Clock, metrics queueMetrics, updatePeriod time.Duration) *Type {
	t := &Type{
		clock:                      c,
		dirty:                      set{},
		processing:                 set{},
		cond:                       sync.NewCond(&sync.Mutex{}),
		metrics:                    metrics,
		unfinishedWorkUpdatePeriod: updatePeriod,
	}
	go t.updateUnfinishedWorkLoop()
	return t
}

const defaultUnfinishedWorkUpdatePeriod = 500 * time.Millisecond

// Type is a work queue (see the package comment).
type Type struct {
	// queue defines the order in which we will work on items. Every
	// element of queue should be in the dirty set and not in the
	// processing set.
	// 用来实现顺序存储元素的
	queue []t

	// dirty defines all of the items that need to be processed.
	// 用来存放需要加入到queue中处理的对象。如果该对象正在处理(processing表中),则不会立即加入到queue中排队，直到处理结束后才加入到队列
	// 即同一个对象如果正在被工作线程处理，则先不排队。当被处理结束后再次排队
	// 用来实现待消费元素的去重
	dirty set

	// Things that are currently being processed are in the processing set.
	// These things may be simultaneously in the dirty set. When we finish
	// processing something and remove it from this set, we'll check if
	// it's in the dirty set, and if so, add it to the queue.
	// 当有对象被从queue提取出来交由工作进程处理时，就会加入到该表中。表明该对象正在被工作进程处理
	// processing 也是用来去重的, 其主要规避元素被并发处理. 当元素还未被处理时, 通过 dirty 来去重,
	// 当前queue只有一个元素, 而当元素已经被执行, 但还未调用Done标记完成。
	// 这时候同一个元素再入队, 会放到 dirty 做去重和排队的效果.
	// 有了 dirty 和 processing 两个集合, queue 队列最大程度去重, 严格避免同一个元素被并发调用, 但会串行调用
	processing set

	cond *sync.Cond // 条件变量。可以广播所有等待者

	// 队列是否关闭
	shuttingDown bool

	metrics queueMetrics

	unfinishedWorkUpdatePeriod time.Duration
	clock                      clock.Clock
}

type (
	empty struct{}
	t     interface{}
	set   map[t]empty
)

func (s set) has(item t) bool {
	_, exists := s[item]
	return exists
}

func (s set) insert(item t) {
	s[item] = empty{}
}

func (s set) delete(item t) {
	delete(s, item)
}

// Add marks item as needing processing.
// 写入一个数据，唤醒一个等待的工作进程.
func (q *Type) Add(item interface{}) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	// 如果队列正在关闭
	if q.shuttingDown {
		return
	}

	// 如果dirty已存在, 则直接退出, dirty是为了实现待消费元素的去重.
	if q.dirty.has(item) {
		return
	}

	q.metrics.add(item)
	// 每次add的元素也要放到 dirty 集合里, 为了去重效果.
	q.dirty.insert(item)
	// 该对象正在被工作线程处理中，则不再加入队列（线程通过Get()提取数据，正在处理)
	// 即如果这个元素正在处理, 那么在把元素放到dirty后就完事了. 后面由Done方法来处理 dirty -> queue 的逻辑.
	if q.processing.has(item) {
		return
	}
	// 加入工作队列排队等待处理
	q.queue = append(q.queue, item)
	// 唤醒一个工作线程
	q.cond.Signal()
}

// Len returns the current queue length, for informational purposes only. You
// shouldn't e.g. gate a call to Add() or Get() on Len() being a particular
// value, that can't be synchronized properly.
// 当前有多少对象等待处理.
func (q *Type) Len() int {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	return len(q.queue)
}

// Get blocks until it can return an item to be processed. If shutdown = true,
// the caller should end their goroutine. You must call Done with item when you
// have finished processing it.
// 如果当前没有数据，而且队列没有关闭，则工作进程等待；如果正在关闭则告知关闭；提取队列第一个对象继续处理.
func (q *Type) Get() (item interface{}, shutdown bool) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	// 如果当前没有数据，而且队列没有关闭，则工作进程等待
	for len(q.queue) == 0 && !q.shuttingDown {
		q.cond.Wait()
	}
	if len(q.queue) == 0 {
		// We must be shutting down.
		return nil, true
	}

	// 从头部获取元素
	item, q.queue = q.queue[0], q.queue[1:]

	q.metrics.get(item)
	// 从 dirty set 里去除, 加到 processing 集合里.
	q.processing.insert(item)
	q.dirty.delete(item)

	return item, false
}

// Done marks item as done processing, and if it has been marked as dirty again
// while it was being processed, it will be re-added to the queue for
// re-processing.
// 某个对象已经被工作线程处理，从processing表中移除；如果该对象又被更新了（需要再次处理），再次将对象新数据进行排队，并唤醒一个工作线程开始处理.
func (q *Type) Done(item interface{}) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	q.metrics.done(item)
	// 从正在处理表中移除
	q.processing.delete(item)
	// 如果该对象又被更新了（需要再次处理），再次将对象新数据进行排队，并唤醒一个工作线程开始处理
	if q.dirty.has(item) {
		q.queue = append(q.queue, item)
		q.cond.Signal()
	}
}

// ShutDown will cause q to ignore all new items added to it. As soon as the
// worker goroutines have drained the existing items in the queue, they will be
// instructed to exit.
// 广播所有等待的工作线程当前队列的关闭.
func (q *Type) ShutDown() {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	q.shuttingDown = true
	q.cond.Broadcast()
}

// 检测当前工作队列是否为空.
func (q *Type) ShuttingDown() bool {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	return q.shuttingDown
}

func (q *Type) updateUnfinishedWorkLoop() {
	t := q.clock.NewTicker(q.unfinishedWorkUpdatePeriod)
	defer t.Stop()
	for range t.C() {
		if !func() bool {
			q.cond.L.Lock()
			defer q.cond.L.Unlock()
			if !q.shuttingDown {
				q.metrics.updateUnfinishedWork()
				return true
			}
			return false
		}() {
			return
		}
	}
}
