package syncer

import (
	"time"

	"github.com/wangweihong/gotoolbox/pkg/wait"
)

type SimpleSyncer struct {
	period     time.Duration
	syncAction func()
}

func NewSimpleSyncer(
	internal time.Duration,
	action func(),
) *SimpleSyncer {
	return &SimpleSyncer{
		period:     internal,
		syncAction: action,
	}
}

func (u *SimpleSyncer) Run(stop <-chan struct{}) {
	go func() {
		wait.Until(u.syncAction, u.period, stop)
	}()
}
