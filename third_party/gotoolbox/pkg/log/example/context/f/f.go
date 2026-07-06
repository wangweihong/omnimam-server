package main

import (
	"time"

	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/gin-gonic/gin"
)

func main() {
	// example1
	gctx := &gin.Context{}
	fields := make(map[string]interface{})
	fields["clientIP"] = "127.0.0.1"
	fields["pid"] = 8888
	fields["startTime"] = time.Now()
	fields["endTime"] = time.Now().Add(2 * time.Hour)

	defer log.Flush()
	ctx := log.WithFields(gctx, fields)
	log.F(ctx).Info("Log with fields")

	opt := log.NewOptions()
	opt.Format = "json"
	// new zapcore.Core
	zapL := log.New(opt)
	defer zapL.Flush()
	jsonL := zapL.WithValuesM(fields)
	jsonL.Info("bbbb")
	jsonL.Info("aaa")
	jsonL.Infof("bb:%v", "aaa")
	jsonL.Infow("bb:", "bbb", "cccc")
	zapL.Info("ccc")
	// L
	gctx2 := &gin.Context{}
	gctx2.Set(string(log.KeyUsername), "127.0.0.1")
	gctx2.Set(string(log.KeyRequestID), "user1")
	log.L(gctx2).Info("Log with fields")
}
