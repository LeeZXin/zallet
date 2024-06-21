package static

import (
	"github.com/spf13/viper"
	"reflect"
)

var (
	v *viper.Viper
)

func Init(baseDir string) {
	// 最终配置
	v = viper.New()
	v.SetConfigType("yaml")
	v.AddConfigPath(baseDir)
	v.SetConfigName("application.yaml")
	_ = v.ReadInConfig()
}

func GetIntSlice(key string) []int {
	return v.GetIntSlice(key)
}

func GetStringSlice(key string) []string {
	return v.GetStringSlice(key)
}

func GetString(key string) string {
	return v.GetString(key)
}

func GetInt(key string) int {
	return v.GetInt(key)
}

func Get(key string) any {
	return v.Get(key)
}

func GetBool(key string) bool {
	return v.GetBool(key)
}

func GetFloat64(key string) float64 {
	return v.GetFloat64(key)
}

func GetStringMapString(key string) map[string]string {
	return v.GetStringMapString(key)
}

func GetStringMap(key string) map[string]any {
	return v.GetStringMap(key)
}

func Exists(key string) bool {
	return v.IsSet(key)
}

func GetInt64(key string) int64 {
	return v.GetInt64(key)
}

func GetMapSlice(key string) []map[string]any {
	ret := Get(key)
	if ret == nil {
		return []map[string]any{}
	}
	r := reflect.ValueOf(ret)
	switch r.Kind() {
	case reflect.Slice, reflect.Array:
	default:
		return []map[string]any{}
	}
	obj := make([]map[string]any, 0, r.Len())
	for i := 0; i < r.Len(); i++ {
		item := r.Index(i).Interface()
		ir := reflect.ValueOf(item)
		if ir.Kind() == reflect.Map && ir.Type().Key().Kind() == reflect.String {
			m := make(map[string]any)
			keys := ir.MapKeys()
			for _, k := range keys {
				m[k.String()] = ir.MapIndex(k).Interface()
			}
			obj = append(obj, m)
		}
	}
	return obj
}
