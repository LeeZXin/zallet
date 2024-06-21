package util

func FindInSlice[T comparable](data []T, target T) bool {
	for _, d := range data {
		if d == target {
			return true
		}
	}
	return false
}
