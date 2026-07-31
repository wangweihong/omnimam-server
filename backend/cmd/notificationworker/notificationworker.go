package main

import notificationworker "github.com/wangweihong/omnimam/backend/internal/notificationworker"

func main() { notificationworker.NewNotificationWorkerApp("notificationworker").Run() }
