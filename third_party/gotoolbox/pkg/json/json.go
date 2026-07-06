package json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

type RawMessage = json.RawMessage

// 根据性能要求切换到其他的编解码器.
var (
	Marshal = json.Marshal
	// 转换JSON的过程中，Unmarshal会按字段名来转换，碰到同名但大小写不一样的字段会当做同一字段处理.
	// To unmarshal JSON into a struct, Unmarshal matches incoming object keys to the keys used by Marshal (either the
	// struct field name or its tag), preferring an exact match but also accepting a case-insensitive match. By default,
	// object keys which don't have a corresponding struct field are ignored (see Decoder.DisallowUnknownFields for an
	// alternative).
	// https://pkg.go.dev/encoding/json#Unmarshal
	Unmarshal     = json.Unmarshal
	MarshalIndent = json.MarshalIndent
	NewDecoder    = json.NewDecoder
	NewEncoder    = json.NewEncoder
)

func PrintStructObject(data any) {
	output, err := json.MarshalIndent(data, "", "\t")
	if err == nil {
		fmt.Println(string(output))
	} else {
		fmt.Println(err)
	}
}

var (
	PrintObject = PrintStructObject
)

// {"hello": "123"}
//
//		-->
//	 {
//		  "hello": "123"
//		}
func PrettyPrint(b []byte) {
	var out bytes.Buffer
	if err := json.Indent(&out, b, "", "  "); err != nil {
		fmt.Println(string(b))
		return
	}
	fmt.Printf("%s\n", out.Bytes())
}

func ToString(obj any) string {
	if obj == nil {
		return ""
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return ""
	}
	return string(b)
}

func Encode(o any) ([]byte, error) {
	return json.Marshal(o)
}

func Decode(y []byte, o any) error {
	return json.Unmarshal(y, o)
}

func ShouldEncode(params any) string {
	b, _ := json.Marshal(params)
	return string(b)
}

func ShouldDecode(b []byte) map[string]any {
	var d map[string]any
	_ = json.Unmarshal([]byte(b), &d)
	return d
}

func ShouldMap(params any) map[string]any {
	b := ShouldEncode(params)
	var d map[string]any
	_ = json.Unmarshal([]byte(b), &d)
	return d
}

// {"source": "\"highlight\""}
// json.Marshal -> {"source": "\\\"highlight\\\""}
// json.RawMarshal -> {"source": "\"highlight\""}
// 场景: elasticsearch通过搜索脚本来搜索时
//
//	{
//		"source": "{ {{#set_highlight}} \"highlight\":{ {{#highlight_field}}
//
// \"fields\":{\"title\":{},\"content\":{\"fragment_size\":\"{{fragment}}\"}} {{/highlight_field}} }, {{/set_highlight}}
// {{#from}} \"from\":\"{{from}}\", {{/from}} {{#size}} \"size\":\"{{size}}\", {{/size}} {{#sort}}
// \"sort\":{{#toJson}}sort{{/toJson}}, {{/sort}}
// \"query\":{\"bool\":{\"must\":[{\"bool\":{\"should\":[{\"term\":{\"space_id\":\"{{space}}\"}}]}},{\"bool\":{{#toJson}}title_content{{/toJson}}}]}}}",
//
//		"params": {
//			"space": 5570565,
//			"fragment": 0,
//			"from": 0,
//			"size": 1,
//			"title_content": {
//				"should": [{
//						"match_phrase": {
//							"title": "超融合"
//						}
//					},
//					{
//						"match_phrase": {
//							"title": "服务器硬件"
//						}
//					},
//					{
//						"match_phrase": {
//							"content": "超融合"
//						}
//					},
//					{
//						"match_phrase": {
//							"content": "服务器硬件"
//						}
//					}
//				]
//			}
//		}
//	}
//
// source是已经转义好的字符串(并根据es的matche语法做了修改), 原生json marshal时会将source的转义字符
// 都加上`\\`,从而导致es matche解析脚本失败。
func RawMarshal(data map[string]any) string {
	md := `{`
	i, length := 0, len(data)
	for k, v := range data {
		md += `"` + k + `":`
		md += ` `
		vt := reflect.TypeOf(v)
		switch vt.Kind() {
		case reflect.Struct, reflect.Map, reflect.Pointer, reflect.Slice:
			md += ShouldEncode(v)
		case reflect.String:
			md += `"` + fmt.Sprintf("%v", v) + `"`
		default:
			md += fmt.Sprintf("%v", v)
		}

		if i != length-1 {
			md += `, `
		}
		i++
	}
	md += `}`
	return md
}

func MapInterfaceDecode(d map[string]any, object any) error {
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, object)
}
