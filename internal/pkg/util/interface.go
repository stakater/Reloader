package util

import (
	"reflect"
	"strconv"

	"github.com/sirupsen/logrus"
)

// InterfaceSlice converts an interface to an interface array
func InterfaceSlice(slice interface{}) []interface{} {
	s := reflect.ValueOf(slice)
	if s.Kind() != reflect.Slice {
		logrus.Errorf("InterfaceSlice() given a non-slice type")
	}

	ret := make([]interface{}, s.Len())

	for i := 0; i < s.Len(); i++ {
		ret[i] = s.Index(i).Interface()
	}

	return ret
}

// ParseBool returns result in bool format after parsing.
// It handles concrete bool/string types as well as any named type whose
// underlying kind is bool or string (e.g. type MyBool bool).
func ParseBool(value interface{}) bool {
	if value == nil {
		return false
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Bool:
		return v.Bool()
	case reflect.String:
		result, _ := strconv.ParseBool(v.String())
		return result
	default:
		return false
	}
}
