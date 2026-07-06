package fieldutil

import (
	"encoding/json"
	"reflect"
)

// 将对象转换成map[string]interface{}
func StructToMap(obj interface{}, hideKey ...string) map[string]interface{} {
	if obj == nil || reflect.TypeOf(obj).Kind() != reflect.Struct {
		return nil
	}

	objMap := make(map[string]interface{})
	b, _ := json.Marshal(obj)
	if b != nil {
		_ = json.Unmarshal(b, &objMap)
		for _, key := range hideKey {
			delete(objMap, key)
		}
	}
	return objMap
}
