package async

// 1. 注册任务类型，该类型任务运行器
// 2. 夹杂去

const (
	TaskStateWaiting = "waiting"
	TaskStateRunning = "running"
	TaskStateFail    = "fail"
	TaskStateSuccess = "success"
)

type TaskRunner any

type Task struct {
	meta interface{}
}

// task
