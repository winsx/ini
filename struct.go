// Copyright 2014 Unknwon
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package ini

import (
	"errors"
	"fmt"
	"reflect"
	"time"
	"unicode"
)

// NameMapper represents a ini tag name mapper.
type NameMapper func(string) string

// Built-in name getters.
var (
	// AllCapsUnderscore converts to format ALL_CAPS_UNDERSCORE.
	AllCapsUnderscore NameMapper = func(raw string) string {
		newstr := make([]rune, 0, 10)
		for i, chr := range raw {
			if isUpper := 'A' <= chr && chr <= 'Z'; isUpper {
				if i > 0 {
					newstr = append(newstr, '_')
				}
			}
			newstr = append(newstr, unicode.ToUpper(chr))
		}
		return string(newstr)
	}
	// TitleUnderscore converts to format title_underscore.
	TitleUnderscore NameMapper = func(raw string) string {
		newstr := make([]rune, 0, 10)
		for i, chr := range raw {
			if isUpper := 'A' <= chr && chr <= 'Z'; isUpper {
				if i > 0 {
					newstr = append(newstr, '_')
				}
				chr -= ('A' - 'a')
			}
			newstr = append(newstr, chr)
		}
		return string(newstr)
	}
)

func (s *Section) parseFieldName(raw, actual string) string {
	if len(actual) > 0 {
		return actual
	}
	if s.f.NameMapper != nil {
		return s.f.NameMapper(raw)
	}
	return raw
}

var reflectTime = reflect.TypeOf(time.Now()).Kind()

func setWithProperType(kind reflect.Kind, key *Key, field reflect.Value) error {
	switch kind {
	case reflect.String:
		field.SetString(key.String())
	case reflect.Bool:
		boolVal, err := key.Bool()
		if err != nil {
			return err
		}
		field.SetBool(boolVal)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := key.Int64()
		if err != nil {
			return err
		}
		field.SetInt(intVal)
	case reflect.Float64:
		floatVal, err := key.Float64()
		if err != nil {
			return err
		}
		field.SetFloat(floatVal)
	case reflectTime:
		timeVal, err := key.Time()
		if err != nil {
			return err
		}
		field.Set(reflect.ValueOf(timeVal))
	case reflect.Slice:
		vals := key.Strings(",")
		numVals := len(vals)
		if numVals == 0 {
			return nil
		}

		sliceOf := field.Type().Elem().Kind()

		var times []time.Time
		if sliceOf == reflectTime {
			times = key.Times(",")
		}

		slice := reflect.MakeSlice(field.Type(), numVals, numVals)
		for i := 0; i < numVals; i++ {
			switch sliceOf {
			case reflectTime:
				slice.Index(i).Set(reflect.ValueOf(times[i]))
			default:
				slice.Index(i).Set(reflect.ValueOf(vals[i]))
			}
		}
		field.Set(slice)
	default:
		return fmt.Errorf("unsupported type '%s'", kind)
	}
	return nil
}

// MapTo maps section to given struct.
func (s *Section) MapTo(val reflect.Value) error {
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := val.Field(i)
		tpField := typ.Field(i)

		tag := tpField.Tag.Get("ini")
		if tag == "-" {
			continue
		}

		fieldName := s.parseFieldName(tpField.Name, tag)
		if len(fieldName) == 0 || !field.CanSet() {
			continue
		}

		if tpField.Type.Kind() == reflect.Struct {
			if sec, err := s.f.GetSection(fieldName); err == nil {
				if err = sec.MapTo(field); err != nil {
					return fmt.Errorf("error mapping field(%s): %v", fieldName, err)
				}
				continue
			}
		} else if tpField.Type.Kind() == reflect.Ptr && tpField.Anonymous {
			field.Set(reflect.New(tpField.Type.Elem()))
			if sec, err := s.f.GetSection(fieldName); err == nil {
				if err = sec.MapTo(field); err != nil {
					return fmt.Errorf("error mapping field(%s): %v", fieldName, err)
				}
				continue
			}
		}

		if key, err := s.GetKey(fieldName); err == nil {
			if err = setWithProperType(tpField.Type.Kind(), key, field); err != nil {
				return fmt.Errorf("error mapping field(%s): %v", fieldName, err)
			}
		}
	}
	return nil
}

// MapTo maps file to given struct.
func (f *File) MapTo(v interface{}) (err error) {
	typ := reflect.TypeOf(v)
	val := reflect.ValueOf(v)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		val = val.Elem()
	} else {
		return errors.New("cannot map to non-pointer struct")
	}

	return f.Section("").MapTo(val)
}

// MapTo maps data sources to given struct with name mapper.
func MapToWithMapper(v interface{}, mapper NameMapper, source interface{}, others ...interface{}) error {
	cfg, err := Load(source, others...)
	if err != nil {
		return err
	}
	cfg.NameMapper = mapper
	return cfg.MapTo(v)
}

// MapTo maps data sources to given struct.
func MapTo(v, source interface{}, others ...interface{}) error {
	return MapToWithMapper(v, nil, source, others...)
}
