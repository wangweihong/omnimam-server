package main

import (
	"time"

	"github.com/wangweihong/gotoolbox/pkg/log"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	defer log.Flush()

	log.Info("info log, int", log.Int("key", 1234))
	log.Info("info log, string", log.String("key", "1234"))
	log.Info("info log, binary", log.Binary("key", []byte("abcd")))
	log.Info("info log, ByteString", log.ByteString("key", []byte("abcd")))
	log.Info("info log, time", log.Time("key", time.Now()))
	r := &request{
		URL:    "/test",
		Listen: addr{"127.0.0.1", 8080},
		Remote: addr{"127.0.0.1", 31200},
	}
	log.Info("info log, object", log.Object("key", r))
	log.Info("info log, any object", log.Any("key", r))
	log.Info("info log, inline object", log.Inline(r))
	rs := requestArray{r, r}
	log.Info("info log, any object array", log.Any("key", rs))
	log.Info("info log,  object array", log.Array("key", rs))

	// infof
	log.Infof("infof log, message: %s", "good")

	// infow
	log.Infow("infow log ", "weather", "good", "date", time.Now().String())
	log.Infow("infow log object", "object", r)
}

type addr struct {
	IP   string
	Port int
}

type request struct {
	URL    string
	Listen addr
	Remote addr
}

func (a addr) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("ip", a.IP)
	enc.AddInt("port", a.Port)
	return nil
}

func (r request) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("url", r.URL)
	zap.Inline(r.Listen).AddTo(enc)
	return enc.AddObject("remote", r.Remote)
}

type requestArray []*request

func (rs requestArray) MarshalLogArray(enc zapcore.ArrayEncoder) error {
	for _, r := range rs {
		_ = enc.AppendObject(r)
	}
	return nil
}
