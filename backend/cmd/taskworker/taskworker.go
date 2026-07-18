package main

import "github.com/wangweihong/omnimam/backend/internal/apiserver"

func main() { apiserver.NewTaskWorkerApp("taskworker").Run() }
