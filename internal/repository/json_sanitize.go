package repository

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"
)

func sanitizeJSONValue(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	switch x := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, val := range x {
			out[k] = sanitizeJSONValue(val)
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(x))
		for _, val := range x {
			out = append(out, sanitizeJSONValue(val))
		}
		return out
	case []map[string]interface{}:
		out := make([]interface{}, 0, len(x))
		for _, val := range x {
			out = append(out, sanitizeJSONValue(val))
		}
		return out
	case map[string]string:
		out := make(map[string]interface{}, len(x))
		for k, val := range x {
			out[k] = sanitizeJSONString(val)
		}
		return out
	case []string:
		out := make([]interface{}, 0, len(x))
		for _, val := range x {
			out = append(out, sanitizeJSONString(val))
		}
		return out
	case string:
		return sanitizeJSONString(x)
	default:
		return sanitizeByReflection(v)
	}
}

func sanitizeJSONString(v string) string {
	if v == "" {
		return v
	}
	clean := strings.ReplaceAll(v, "\x00", "")
	if utf8.ValidString(clean) {
		return clean
	}
	return strings.ToValidUTF8(clean, "")
}

func sanitizeByReflection(v interface{}) interface{} {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return nil
	}

	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface:
		if rv.IsNil() {
			return nil
		}
		return sanitizeByReflection(rv.Elem().Interface())
	case reflect.Struct:
		raw, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		var out interface{}
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil
		}
		return sanitizeJSONValue(out)
	case reflect.Slice, reflect.Array:
		length := rv.Len()
		out := make([]interface{}, 0, length)
		for i := 0; i < length; i++ {
			out = append(out, sanitizeJSONValue(rv.Index(i).Interface()))
		}
		return out
	case reflect.Map:
		out := make(map[string]interface{}, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			key := sanitizeJSONString(strings.TrimSpace(toString(iter.Key().Interface())))
			out[key] = sanitizeJSONValue(iter.Value().Interface())
		}
		return out
	default:
		return v
	}
}

func toString(v interface{}) string {
	return strings.TrimSpace(strings.ReplaceAll(fmt.Sprint(v), "\x00", ""))
}
