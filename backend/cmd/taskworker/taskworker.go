package main

import taskworker "github.com/wangweihong/omnimam/backend/internal/taskworker"

func main() { taskworker.NewTaskWorkerApp("taskworker").Run() }
