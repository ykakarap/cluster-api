/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"fmt"
	"reflect"
)

const (
	fieldStatus            = "Status"
	fieldMessage           = "Message"
	fieldRetryAfterSeconds = "RetryAfterSeconds"
)

func valueOf(obj interface{}) (reflect.Value, error) {
	var v reflect.Value
	var t reflect.Type
	if reflect.TypeOf(obj).Kind() == reflect.Ptr {
		t = reflect.TypeOf(obj).Elem()
		v = reflect.ValueOf(obj).Elem()
	} else {
		t = reflect.TypeOf(obj)
		v = reflect.ValueOf(obj)
	}
	if t.Kind() != reflect.Struct {
		return v, fmt.Errorf("unexpected type")
	}
	return v, nil
}

func IsFailure(obj interface{}) (bool, error) {
	status, err := GetStatus(obj)
	if err != nil {
		return false, err
	}
	return status == ResponseStatusFailure, nil
}

func GetStatus(obj interface{}) (ResponseStatus, error) {
	status, err := getString(obj, fieldStatus)
	return ResponseStatus(status), err
}

func SetStatus(obj interface{}, status ResponseStatus) error {
	return setString(obj, string(status), fieldStatus)
}

func GetMessage(obj interface{}) (string, error) {
	return getString(obj, fieldMessage)
}

func SetMessage(obj interface{}, msg string) error {
	return setString(obj, msg, fieldMessage)
}

func GetRetryAfterSeconds(obj interface{}) (int, error) {
	return getInt(obj, fieldRetryAfterSeconds)
}

func SetRetryAfterSeconds(obj interface{}, i int) error {
	return setInt(obj, i, fieldRetryAfterSeconds)
}

func getString(obj interface{}, field string) (string, error) {
	v, err := valueOf(obj)
	if err != nil {
		return "", err
	}
	f := v.FieldByName(field)
	if !f.IsValid() {
		return "", fmt.Errorf("unexpected type. filed %s does not exist", field)
	}
	s, ok := f.Interface().(string)
	if !ok {
		return "", fmt.Errorf("unexpected type. field %s is of wrong type", field)
	}
	return s, nil
}

func setString(obj interface{}, s string, field string) error {
	v, err := valueOf(obj)
	if err != nil {
		return err
	}
	f := v.FieldByName(field)
	if !f.CanSet() {
		return fmt.Errorf("unexpected type. cannot set field %s", field)
	}
	f.SetString(s)
	return nil
}

func getInt(obj interface{}, field string) (int, error) {
	v, err := valueOf(obj)
	if err != nil {
		return 0, err
	}
	f := v.FieldByName(field)
	if !f.IsValid() {
		return 0, fmt.Errorf("unexpected type. filed %s does not exist", field)
	}
	i, ok := f.Interface().(int)
	if !ok {
		return 0, fmt.Errorf("unexpected type. field %s is of wrong type", field)
	}
	return i, nil
}

func setInt(obj interface{}, i int, field string) error {
	v, err := valueOf(obj)
	if err != nil {
		return err
	}
	f := v.FieldByName(field)
	if !f.CanSet() {
		return fmt.Errorf("unexpected type. cannot set field %s", field)
	}
	f.SetInt(int64(i))
	return nil
}
