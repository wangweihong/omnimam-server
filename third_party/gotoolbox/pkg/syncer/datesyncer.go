package syncer

import (
	"sync"
	"time"
)

// OneWorkerDataSyncer only one worker run.
type OneWorkerDataSyncer struct {
	*OneWorkerSyncer
	data     any
	dataLock sync.RWMutex
}

func NewOneWorkerDataSyncer(
	syncAction func(arg any) (any, error),
	internal time.Duration,
	keepResultNum int,
) *OneWorkerDataSyncer {
	if internal == 0 {
		internal = 30 * time.Second
	}
	u := &OneWorkerDataSyncer{}
	workerFunc := func(arg any) error {
		data, err := syncAction(arg)
		if err != nil {
			return err
		}

		u.dataLock.Lock()
		defer u.dataLock.Unlock()
		u.data = data
		return nil
	}

	worker := NewOneWorkerSyncer(workerFunc, internal, keepResultNum)
	u.OneWorkerSyncer = worker

	return u
}

func (s *OneWorkerDataSyncer) Get() any {
	s.dataLock.RLock()
	defer s.dataLock.RUnlock()

	return s.data
}
