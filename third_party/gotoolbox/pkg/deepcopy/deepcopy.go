package deepcopy

func AnyMapClone(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		switch typed := value.(type) {
		case map[string]any:
			result[key] = AnyMapClone(typed)
		case []any:
			items := make([]any, len(typed))
			for i, item := range typed {
				if nested, ok := item.(map[string]any); ok {
					items[i] = AnyMapClone(nested)
				} else {
					items[i] = item
				}
			}
			result[key] = items
		default:
			result[key] = value
		}
	}
	return result
}