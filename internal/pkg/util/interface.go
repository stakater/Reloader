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

// ParseBool returns result in bool format after parsing
func ParseBool(value interface{}) bool {
	if reflect.Bool == reflect.TypeOf(value).Kind() {
		b, _ := value.(bool)
		return b
	} else if reflect.String == reflect.TypeOf(value).Kind() {
		s, _ := value.(string)
		result, _ := strconv.ParseBool(s)
		return result
	}
	return false
}
