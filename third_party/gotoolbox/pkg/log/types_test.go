package log_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/kr/pretty"
	"go.uber.org/zap/zapcore"
)

type InnerStruct struct {
	FieldA string
	FieldB int
}

type MyStruct struct {
	Field1 int
	Field2 string
	Field3 *InnerStruct
}

func TestEvery(t *testing.T) {
	//{"slice": "[]string{\"a lll\"}"}
	log.Info("", log.Every("slice", []string{"a lll"}))
	//{"map": "map[string]interface {}{\"aaa\":123, \"name\":false, \"so\":\"bb\"}"}
	log.Info("", log.Every("map", map[string]any{"aaa": 123, "so": "bb", "name": false}))
	type Person struct {
		Emails []string `mapstructure:"emails"`
	}
	var result Person
	result.Emails = []string{"a", "b"}

	var result2 Person
	result2.Emails = []string{"a", "b"}
	// {"object": "log_test.Person{Emails:[]string{\"a\", \"b\"}}"}
	log.Info("", log.Every("object", result))
	// {"objectP":"&log_test.Person{Emails:[]string{\"a\", \"b\"}}"}
	log.Info("", log.Every("objectP", &result))
	// {"objectP":"[]log_test.Person{log_test.Person{Emails:[]string{\"a\", \"b\"}},
	// log_test.Person{Emails:[]string{\"a\", \"b\"}}}"}
	log.Info("", log.Every("objectP", []Person{result, result2}))

	// 创建一个包含指针字段的结构体
	data := &MyStruct{
		Field1: 42,
		Field2: "Hello",
		Field3: &InnerStruct{
			FieldA: "World",
			FieldB: 100,
		},
	}
	//{"data": "{\"Field1\":42,\"Field2\":\"Hello\",\"Field3\":{\"FieldA\":\"World\",\"FieldB\":100}}"}
	log.Info("", log.Every("objectP", data))
	log.Info("", log.Every("int", 1))
	log.Info("", log.Every("string", "abc"))
	// {"time": "\"2023-08-03T09:54:09.6885847+08:00\""}
	log.Info("", log.Every("time", time.Now()))
	log.Info("", log.Every("string slice", []string{"aa", "bb"}))
	log.Info("", log.Every("map", map[string]*MyStruct{"aa": data}))

}

func TestPretty(t *testing.T) {
	data := &MyStruct{
		Field1: 42,
		Field2: "Hello",
		Field3: &InnerStruct{
			FieldA: "World",
			FieldB: 100,
		},
	}
	// {"data":
	// "&log_test.MyStruct{Field1:42,Field2:\"Hello\",Field3:&log_test.InnerStruct{FieldA:\"World\",FieldB:100},} "}
	log.Info("", log.Pretty("objectP", data))
	log.Info("", log.Pretty("int", 123))
	log.Info("", log.Pretty("string", "456"))
	//{"time": "time.Date(2023,time.August,3,9,52,44,419798400,time.Local) "}
	log.Info("", log.Pretty("time", time.Now()))
	log.Info("", log.Pretty("string slice", []string{"aa", "bb"}))
	// {"map":
	// "map[string]*log_test.MyStruct{\"aa\":&log_test.MyStruct{Field1:42,Field2:\"Hello\",Field3:&log_test.InnerStruct{FieldA:\"World\",FieldB:100},},}
	// "}
	log.Info("", log.Pretty("map", map[string]*MyStruct{"aa": data}))
	//fmt.Printf("%# v", pretty.Formatter(data))

	str := fmt.Sprintf("%# v", pretty.Formatter(data))
	log.Info("", log.String("map", str))

}

func benchStartup() *MyStruct {
	// 创建一个包含指针字段的结构体
	data := &MyStruct{
		Field1: 42,
		Field2: "Hello",
		Field3: &InnerStruct{
			FieldA: "World",
			FieldB: 100,
		},
	}
	opts := log.NewOptions()
	opts.OutputPaths = nil
	opts.ErrorOutputPaths = nil
	// 初始化全局logger
	log.Init(opts)

	return data
}

func every(key string, val any) log.Field {
	str := fmt.Sprintf("%#v", val)
	return log.Field{Key: key, Type: zapcore.StringType, String: str}
}

// BenchmarkOldEvery-4      1252472               925.7 ns/op
func BenchmarkOldEvery(b *testing.B) {
	data := benchStartup()
	defer log.Flush()

	// 重置计时器
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		log.Info("", every("data", data))
	}
}

// BenchmarkEvery-4        2054840               575.8 ns/o
func BenchmarkEvery(b *testing.B) {
	data := benchStartup()
	defer log.Flush()

	b.ResetTimer()
	// 运行基准测试
	for i := 0; i < b.N; i++ {
		log.Info("", log.Every("data", data))
	}
}

// BenchmarkAny-4           6892203               173.7 ns/op
func BenchmarkAny(b *testing.B) {
	data := benchStartup()
	defer log.Flush()
	// 重置计时器
	b.ResetTimer()

	// 运行基准测试
	for i := 0; i < b.N; i++ {
		log.Info("", log.Any("data", data))
	}
}

// BenchmarkString-4        6473648               177.9 ns/o
func BenchmarkString(b *testing.B) {
	data := benchStartup()
	defer log.Flush()
	d, _ := json.Marshal(data)

	// 重置计时器
	b.ResetTimer()

	// 运行基准测试
	for i := 0; i < b.N; i++ {
		log.Info("", log.String("data", string(d)))
	}
}

func BenchmarkPretty(b *testing.B) {
	data := benchStartup()

	defer log.Flush()
	// 重置计时器
	b.ResetTimer()

	// 运行基准测试
	for i := 0; i < b.N; i++ {
		log.Info("", log.Pretty("data", data))
	}
}
