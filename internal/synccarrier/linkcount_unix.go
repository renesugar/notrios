//go:build !windows && !plan9 && !js

package synccarrier

import (
	"os"
	"reflect"
)

func singleLink(_ *os.File, info os.FileInfo) bool {
	value := reflect.ValueOf(info.Sys())
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return false
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return false
	}
	field := value.FieldByName("Nlink")
	if !field.IsValid() {
		return false
	}
	switch field.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return field.Uint() == 1
	default:
		return false
	}
}
