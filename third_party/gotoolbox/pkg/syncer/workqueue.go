package syncer

import (
	"sync"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/sequential"
	"github.com/wangweihong/gotoolbox/pkg/wait"
	"github.com/wangweihong/gotoolbox/pkg/workqueue"
)

type WorkequeueSyncer struct {
	period      time.Duration
	stopCh      <-chan struct{}
	syncAction  func(arg any) error
	syncResult  *sequential.List
	queue       workqueue.Interface
	threadiness int

	lock sync.RWMutex
}

func NewWorkequeueSyncer(
	// 如果action为更新对象之类的动作, 应通过arg传递对象ID
	// action根据id获取最新的对象, 再进行更新。
	// 因为同一个对象多次插入队列(dirty/process), 后续的插入会被忽略
	action func(arg any) error,
	queue workqueue.Interface,
	internal time.Duration,
	threadiness int,
	keepResultNum int,
) *WorkequeueSyncer {
	if threadiness == 0 {
		threadiness = 1
	}
	s := &WorkequeueSyncer{
		period:      internal,
		syncAction:  action,
		syncResult:  sequential.NewLimitSequentialList(keepResultNum),
		queue:       queue,
		threadiness: threadiness,
	}

	return s
}

// Run 运行后台定时同步器.
func (u *WorkequeueSyncer) Run(stop <-chan struct{}) {
	go func() {
		// 停止时关闭掉
		defer u.queue.ShutDown()

		// 从协程池中运行消费者
		for i := 0; i < u.threadiness; i++ {
			// 之所以使用wait.Until来执行消费者是runWorker有可能会panic
			// wait.Unit会panic recover, 然后重建消费者线程。
			go wait.Until(u.runWorker, time.Second, stop)
		}

		<-stop
	}()
}

// Trigger trigger syncer action.
func (u *WorkequeueSyncer) Trigger(arg any, auto bool) bool {
	u.queue.Add(arg)
	return false
}

func (u *WorkequeueSyncer) runWorker() {
	for u.processNextItem() {
	}
}

func (u *WorkequeueSyncer) processNextItem() bool {
	// 如果工作队列没有消费数据, 消费线程均会阻塞在这里
	// 直到有消费元素，会唤醒阻塞的一个线程进行消费
	key, quit := u.queue.Get()
	if quit {
		return false
	}

	// 告诉队列我们已经完成了处理此 key 的操作
	// 这将为其他 worker 解锁该 key
	// 这将确保安全的并行处理，因为永远不会并行处理具有相同 key 的两个Pod
	defer u.queue.Done(key)

	index := u.startRecord(true, key)
	err := u.syncAction(key)

	u.handleErr(err, key)
	u.finishRecord(index, err)
	return true
}

// 检查是否发生错误，并确保我们稍后重试.
func (u *WorkequeueSyncer) handleErr(err error, key any) {
	if err == nil {
		return
	}
	log.Errorf("sync %v error:%v", key, err)
}

func (u *WorkequeueSyncer) GetRecords() []SyncInfo {
	u.lock.RLock()
	defer u.lock.RUnlock()

	rs := make([]SyncInfo, 0, u.syncResult.Len())
	for _, v := range u.syncResult.List() {
		rs = append(rs, v.(SyncInfo))
	}
	return rs
}

func (u *WorkequeueSyncer) startRecord(auto bool, key any) int {
	u.lock.Lock()
	defer u.lock.Unlock()

	return u.syncResult.Inject(*NewSyncInfo(auto, key))
}

func (u *WorkequeueSyncer) finishRecord(i int, err error) {
	u.lock.Lock()
	defer u.lock.Unlock()

	data := u.syncResult.Get(i)
	si := data.(SyncInfo)

	si.Finish(err)
	u.syncResult.Update(i, si)
}
