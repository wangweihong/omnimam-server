package syncer

import (
	"sync"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/sequential"
	"github.com/wangweihong/gotoolbox/pkg/wait"
)

// OneWorkerSyncer only one worker run.
type OneWorkerSyncer struct {
	period     time.Duration
	working    bool
	syncAction func(arg any) error
	syncResult *sequential.List

	lock sync.RWMutex
}

func NewOneWorkerSyncer(
	syncAction func(arg any) error,
	internal time.Duration,
	keepResultNum int,
) *OneWorkerSyncer {
	if internal == 0 {
		internal = 30 * time.Second
	}

	u := &OneWorkerSyncer{
		period:     internal,
		syncAction: syncAction,
		syncResult: sequential.NewLimitSequentialList(keepResultNum),
	}

	return u
}

// Run start a period syncer in backend.
func (u *OneWorkerSyncer) Run(stop <-chan struct{}) {
	go func() {
		wait.Until(func() {
			u.Trigger(nil, true)
		}, u.period, stop)
	}()
}

// Trigger trigger syncer action.
func (u *OneWorkerSyncer) Trigger(arg any, auto bool) bool {
	u.lock.Lock()
	defer u.lock.Unlock()

	if u.working {
		return true
	}
	u.working = true
	go u.processNextItem(auto, arg)
	return false
}

func (u *OneWorkerSyncer) processNextItem(auto bool, key any) {
	defer func() {
		u.lock.Lock()
		defer u.lock.Unlock()

		u.working = false
	}()

	index := u.startRecord(auto, key)
	// 调用包含业务逻辑的方法
	err := u.syncAction(key)

	// 如果在执行业务逻辑期间出现错误，则处理错误
	u.handleErr(err, key)
	u.finishRecord(index, err)
}

func (u *OneWorkerSyncer) handleErr(err error, key any) {
	if err == nil {
		return
	}
	log.Errorf("sync %v error:%v", key, err)
}

func (u *OneWorkerSyncer) GetRecords() []SyncInfo {
	u.lock.RLock()
	defer u.lock.RUnlock()

	rs := make([]SyncInfo, 0, u.syncResult.Len())
	for _, v := range u.syncResult.List() {
		rs = append(rs, v.(SyncInfo))
	}
	return rs
}

func (u *OneWorkerSyncer) startRecord(auto bool, key any) int {
	u.lock.Lock()
	defer u.lock.Unlock()

	return u.syncResult.Inject(*NewSyncInfo(auto, key))
}

func (u *OneWorkerSyncer) finishRecord(i int, err error) {
	u.lock.Lock()
	defer u.lock.Unlock()

	data := u.syncResult.Get(i)
	si := data.(SyncInfo)

	si.Finish(err)
	u.syncResult.Update(i, si)
}
