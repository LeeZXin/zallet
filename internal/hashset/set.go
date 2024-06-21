package hashset

type HashSet[T comparable] struct {
	m map[T]struct{}
}

func NewHashSet[T comparable](t ...T) *HashSet[T] {
	ret := &HashSet[T]{
		m: make(map[T]struct{}),
	}
	ret.Add(t...)
	return ret
}

func (s *HashSet[T]) Add(ts ...T) {
	for _, t := range ts {
		s.m[t] = struct{}{}
	}
}

func (s *HashSet[T]) Remove(ts ...T) {
	for _, t := range ts {
		delete(s.m, t)
	}
}

func (s *HashSet[T]) Contains(t T) bool {
	_, b := s.m[t]
	return b
}

func (s *HashSet[T]) AllKeys() []T {
	ret := make([]T, 0, len(s.m))
	for t, _ := range s.m {
		ret = append(ret, t)
	}
	return ret
}

func (s *HashSet[T]) Size() int {
	return len(s.m)
}

func (s *HashSet[T]) Range(fn func(T)) {
	for t := range s.m {
		fn(t)
	}
}
