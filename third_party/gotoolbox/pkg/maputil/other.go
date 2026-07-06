package maputil

// GetStringFromMapInterface 从对象中提取字符串
// 支持以下：
//
//	{"data":"string"}
//	{"data":{"data":"string"}}
//	{"data":{"child":{data:"string"}}}
func GetStringFromMapInterface(param map[string]any, datakey string, parent ...string) string {
	if param == nil || datakey == "" {
		return ""
	}
	subParam := param

	for i := 0; i < len(parent); i++ {
		if subParam == nil {
			return ""
		}
		d := subParam[parent[i]]
		if d != nil {
			dv, _ := d.(map[string]any)
			subParam = dv
		}
	}
	d := subParam[datakey]
	if d != nil {
		dv, _ := d.(string)
		return dv
	}

	return ""
}

func GetFloat64FromMapInterface(param map[string]any, datakey string, parent ...string) float64 {
	if param == nil || datakey == "" {
		return 0
	}
	subParam := param

	for i := 0; i < len(parent); i++ {
		if subParam == nil {
			return 0
		}
		d := subParam[parent[i]]
		if d != nil {
			dv, _ := d.(map[string]any)
			subParam = dv
		}
	}
	d := subParam[datakey]
	if d != nil {
		dv, _ := d.(float64)
		return dv
	}
	return 0
}

func GetMapSliceFromMapInterface(param map[string]any, datakey string, parent ...string) []map[string]any {
	if param == nil || datakey == "" {
		return nil
	}
	subParam := param

	for i := 0; i < len(parent); i++ {
		if subParam == nil {
			return nil
		}
		d := subParam[parent[i]]
		if d != nil {
			dv, _ := d.(map[string]any)
			subParam = dv
		}
	}

	d := subParam[datakey]
	dv, _ := d.([]any)
	if dv != nil {
		m := make([]map[string]any, 0, len(dv))
		for _, v := range dv {
			vd, _ := v.(map[string]any)
			if vd != nil {
				m = append(m, vd)
			}
		}
		return m
	}
	return nil
}

// GetMapFromMapInterface 从对象中提取map[string]interface
// 支持以下：
//
//	{"data":{"child":["string"]}}
//	{"data":{"child":"string"}}
//	{"data":{"child":{child2:"string"}}}
//	{"data":{"child":[{child2:"string"}]}}
func GetMapFromMapInterface(param map[string]any, datakey string, parent ...string) map[string]any {
	if param == nil || datakey == "" {
		return nil
	}
	subParam := param

	for i := 0; i < len(parent); i++ {
		if subParam == nil {
			return nil
		}
		d := subParam[parent[i]]
		if d != nil {
			dv, _ := d.(map[string]any)
			subParam = dv
		}

	}
	d := subParam[datakey]
	if d != nil {
		dv, _ := d.(map[string]any)
		return dv
	}
	return nil
}

// GetStringSliceFromMapInterface 从对象中提取字符串列表
// 支持以下：
//
//	{"data":"string"}
//	{"data":["string"]}
//	{"data":{"child":["string"]}}
//	{"data":{"child":"string"}}
//	{"data":{"child":{child2:"string"}}}
//	{"data":{"child":[{child2:"string"}]}}
func GetStringSliceFromMapInterface(param map[string]any, dataKey string, parents ...string) []string {
	if param == nil || dataKey == "" {
		return nil
	}

	var data []string

	subParam := param
	if parents == nil {
		di := param[dataKey]
		if di != nil {
			switch di.(type) {
			case string:
				data = append(data, di.(string))
			case []any:
				for _, v := range di.([]any) {
					if vd, ok := v.(string); ok {
						data = append(data, vd)
					}
				}
			}
		}
		return data
	}

	for i := 0; i < len(parents); i++ {
		if subParam == nil {
			return nil
		}
		if i != len(parents)-1 {
			d, _ := subParam[parents[i]]
			if d != nil {
				dv, _ := d.(map[string]any)
				subParam = dv
			}
			continue
		}

		// last parent
		di, _ := subParam[parents[i]]
		if di != nil {
			switch di.(type) {
			case map[string]any:
				dki, _ := di.(map[string]any)[dataKey]
				if dki != nil {
					switch dki.(type) {
					case string:
						data = append(data, dki.(string))
					case []any:
						for _, v := range dki.([]any) {
							if _, ok := v.(string); ok {
								data = append(data, v.(string))
							}
						}
					}
				}
			case []any:
				for _, dsi := range di.([]any) {
					dsd, _ := dsi.(map[string]any)
					if dsd != nil {
						dki, _ := dsd[dataKey]
						if dki != nil {
							switch dki.(type) {
							case string:
								data = append(data, dki.(string))
							case []any:
								for _, v := range dki.([]any) {
									if _, ok := v.(string); ok {
										data = append(data, v.(string))
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return data
}
