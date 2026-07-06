package general

func FallbackIfNil[T any](ptr *T, fallback T) T {
	if ptr != nil {
		return *ptr
	}
	return fallback
}

func FallbackIfEmpty[T string](ptr T, fallback T) T {
	if ptr != "" {
		return ptr
	}
	return fallback
}

func FallbackIfMatch[T any](ptr T, fallback T, fn func(T) bool) T {
	if fn(ptr) {
		return ptr
	}
	return fallback
}
