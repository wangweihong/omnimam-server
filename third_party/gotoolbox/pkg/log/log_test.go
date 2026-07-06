package log_test

import (
	"context"
	"errors"
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/log"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func Test_WithName(t *testing.T) {
	defer log.Flush() // used for record logger printer

	logger := log.WithName("test")
	logger.Infow("Hello world!", "foo", "bar") // structed logger
}

func Test_WithValues(t *testing.T) {
	defer log.Flush() // used for record logger printer

	logger := log.WithValues("key", "value") // used for record context
	logger.Info("Hello world!")
	logger.Info("Hello world!")

	logger2 := log.WithValues("value") // used for record context
	logger2.Info("Hello world!")

}

func TestZapLogger_WithValuesM(t *testing.T) {
	defer log.Flush() // used for record logger printer

	f := make(map[string]any)
	f["key"] = "value"
	logger := log.WithValuesM(f) // used for record context
	logger.Info("Hello world!")
	logger.Info("Hello world!")

	f["key2"] = ""
	ctx := log.WithFields(context.Background(), f)
	log.F(ctx).Info("s")

}

func Test_V(t *testing.T) {
	defer log.Flush() // used for record logger printer

	log.V(0).Infow("Hello world!", "key", "value")
	log.V(1).Infow("Hello world!", "key", "value")
}

func Test_Option(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ExitOnError)
	opt := log.NewOptions()
	opt.AddFlags(fs)

	args := []string{"--log.level=debug"}
	err := fs.Parse(args)
	assert.Nil(t, err)

	assert.Equal(t, "debug", opt.Level)
}

func Test_F(t *testing.T) {
	defer log.Flush()

	fields := make(map[string]any, 0)
	fields["traceID"] = "12345678"
	fields["name"] = "libai"
	ctx := log.WithFields(context.Background(), fields)

	// Log with fields	{"name": "libai", "traceID": "12345678"}
	log.F(ctx).Info("Log with fields")

	field2 := make(map[string]any, 0)
	field2["other"] = "aaa"
	ctx = log.WithFields(ctx, field2)
	d := ctx.Value(log.FieldKeyCtx{})
	if d == nil {
		t.Log("field is nil")
		t.Fail()
	}

	dm := d.(map[string]any)
	if _, ok := dm["name"]; !ok {
		t.Log("name not exist")
		t.Fail()
	}

	if len(dm) != 3 {
		t.Log("len not match")
		t.Fail()
	}

	// Log with fields	{"name": "libai", "other": "aaa", "traceID": "12345678"}
	log.F(ctx).Info("Log with fields")
}

func Test_L(t *testing.T) {
	defer log.Flush()

	ctx := context.WithValue(context.Background(), log.KeyRequestID, "12345678")
	ctx = context.WithValue(ctx, log.KeyUsername, "libai")
	// Log with L	{"requestID": "12345678", "username": "libai"}
	log.L(ctx).Info("Log with L")
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

func Test_ZapType(t *testing.T) {
	defer log.Flush()

	log.Info("This is Int example", log.Int("int_key", 10))
	log.Info("This is Any:Int example", log.Any("int_key", 123))

	log.Info("This is Err example", log.Err(errors.New("my error")))
	log.Info("This is Any:Err example", log.Any("error", errors.New("my error")))

	req := &request{
		URL:    "/test",
		Listen: addr{"127.0.0.1", 8080},
		Remote: addr{"127.0.0.1", 31200},
	}
	log.Info("This is object example", zap.Object("req", req))
	log.Info("This is inline:object example", zap.Inline(req))
	log.Info("This is Any:object example", log.Any("req", req))
	_, _ = zap.NewDevelopment()
}

// Benchmark_ZapTypeAny-4           6664069               181.7 ns/op
func Benchmark_ZapTypeAny(b *testing.B) {
	opt := log.NewOptions()
	opt.OutputPaths = nil
	opt.ErrorOutputPaths = nil
	log.Init(opt)
	defer log.Flush()

	req := &request{
		URL:    "/test",
		Listen: addr{"127.0.0.1", 8080},
		Remote: addr{"127.0.0.1", 31200},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("This is Any:object example", log.Any("req", req))
	}
}

// Benchmark_ZapTypeObject-4        6823978               169.4 ns/op
func Benchmark_ZapTypeObject(b *testing.B) {
	opt := log.NewOptions()
	opt.OutputPaths = nil
	opt.ErrorOutputPaths = nil
	log.Init(opt)
	defer log.Flush()

	req := &request{
		URL:    "/test",
		Listen: addr{"127.0.0.1", 8080},
		Remote: addr{"127.0.0.1", 31200},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("This is object example", log.Object("req", req))
	}
}
