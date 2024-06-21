package util

func Filter[T any](data []T, fn func(T) bool) []T {
	if fn == nil {
		return nil
	}
	ret := make([]T, 0)
	for _, d := range data {
		b := fn(d)
		if b {
			ret = append(ret, d)
		}
	}
	return ret
}

func Map[T, K any](data []T, mapper func(T) K) []K {
	if mapper == nil {
		return nil
	}
	ret := make([]K, 0, len(data))
	for _, d := range data {
		k := mapper(d)
		ret = append(ret, k)
	}
	return ret
}
