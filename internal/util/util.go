package util

func MergeEnvs(with map[string]string) []string {
	ret := make([]string, 0, len(with))
	for k, v := range with {
		ret = append(ret, k+"="+v)
	}
	return ret
}
