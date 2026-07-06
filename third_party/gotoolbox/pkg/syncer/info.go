package syncer

import (
	"time"

	"github.com/wangweihong/gotoolbox/pkg/typeutil"
)

const (
	StateExecuting = "executing"
	StateFailed    = "failed"
	StateSuccess   = "success"
)

type SyncInfo struct {
	StartTime time.Time
	EndTime   *time.Time
	// 自动执行还是手动执行
	Auto    bool
	Fail    bool
	Message string
	// record trigger
	Key any
}

func NewSyncInfo(auto bool, key any) *SyncInfo {
	return &SyncInfo{
		StartTime: time.Now(),
		EndTime:   nil,
		Auto:      auto,
		Fail:      false,
		Message:   "",
		Key:       key,
	}
}

func (si *SyncInfo) Finish(err error) {
	si.EndTime = typeutil.Time(time.Now())
	if err != nil {
		si.Fail = true
		si.Message = err.Error()
	}
}
