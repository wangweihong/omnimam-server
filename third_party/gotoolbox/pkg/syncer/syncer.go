package syncer

type Action func(arg any) error

type Service interface {
	Run(stop <-chan struct{})
	Trigger(arg any, auto bool) bool
	GetRecords() []SyncInfo
}

var (
	_ Service = (*OneWorkerSyncer)(nil)
	_ Service = (*WorkequeueSyncer)(nil)
)
