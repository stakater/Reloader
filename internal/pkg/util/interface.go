package util

import (
	"reflect"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type ObjectMeta struct {
	metav1.ObjectMeta
}

func ToObjectMeta(kubernetesObject interface{}) ObjectMeta {
	objectValue := reflect.ValueOf(kubernetesObject)
	fieldName := reflect.TypeOf((*metav1.ObjectMeta)(nil)).Elem().Name()
	field := objectValue.FieldByName(fieldName).Interface().(metav1.ObjectMeta)

	return ObjectMeta{
		ObjectMeta: field,
	}
}
