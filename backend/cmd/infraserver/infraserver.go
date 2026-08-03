package main

import "github.com/wangweihong/omnimam/backend/internal/infrastructure"

func main() { infrastructure.NewApp("infraserver").Run() }
