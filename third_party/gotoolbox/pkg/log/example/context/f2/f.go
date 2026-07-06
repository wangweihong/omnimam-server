package main

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/wangweihong/gotoolbox/pkg/log"
)

func main() {
	// save key/value pair in context
	ctx := log.WithFieldPair(context.Background(), "ip", "10.30.100.111")

	// save logger in context
	ctx = log.F(ctx).WithContext(ctx)

	// every log will carry ip:10.30.100.111
	log.FromContext(ctx).Info("bbb")
	log.FromContext(ctx).Info("okok", log.String("name", "bbb"))

	// example: how to use fields in gin context
	fields := make(map[string]any)
	fields["name"] = "bob"
	fields["age"] = 17

	gctx := &gin.Context{}
	gctx.Set(log.FieldKeyCtx{}.String(), fields)
	// 2023-07-19 17:24:24.776 INFO    f2/f.go:27      gin context example     {"name": "bob", "age": 17}
	log.F(gctx).Info("gin context example")

	gctx.Set(string(log.KeyRequestID), "123456")
	// 2023-07-19 17:27:39.820 INFO    f2/f.go:32      gin context example 2   {"name": "bob", "age": 17, "requestID":
	// "123456"}
	log.F(gctx).L(gctx).Info("gin context example 2")
}
