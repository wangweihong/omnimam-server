package syncx

func Range[T any](s []T, limit int, fn func(val T) error) error {
	lg := NewRateLimitGroup(limit)
	for _, val := range s {
		val := val

		lg.SafeGoError(func() error { return fn(val) })
	}
	return lg.WaitError()
}