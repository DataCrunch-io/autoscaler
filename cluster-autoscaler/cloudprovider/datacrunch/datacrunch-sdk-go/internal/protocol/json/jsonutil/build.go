package jsonutil

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
)

const (
	floatNaN    = "NaN"
	floatInf    = "Infinity"
	floatNegInf = "-Infinity"
)

// BuildJSON builds a JSON string for a given object v with locationName support.
func BuildJSON(v interface{}) ([]byte, error) {
	return json.Marshal(locationNameWrapper{v})
}

type locationNameWrapper struct {
	value interface{}
}

func (w locationNameWrapper) MarshalJSON() ([]byte, error) {
	return marshalWithLocationName(reflect.ValueOf(w.value))
}

func marshalWithLocationName(v reflect.Value) ([]byte, error) {
	v = reflect.Indirect(v)
	if !v.IsValid() {
		return json.Marshal(nil)
	}

	if v.Kind() != reflect.Struct {
		// Handle special cases for non-struct types
		switch val := v.Interface().(type) {
		case float64:
			if math.IsNaN(val) {
				return []byte("\"" + floatNaN + "\""), nil
			} else if math.IsInf(val, 1) {
				return []byte("\"" + floatInf + "\""), nil
			} else if math.IsInf(val, -1) {
				return []byte("\"" + floatNegInf + "\""), nil
			}
		}
		return json.Marshal(v.Interface())
	}

	// Build a map with locationName-mapped field names
	result := make(map[string]interface{})
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		if !fieldValue.CanInterface() {
			continue
		}
		if field.Tag.Get("json") == "-" {
			continue
		}
		if field.Tag.Get("location") != "" {
			continue // ignore non-body elements
		}
		if field.Tag.Get("ignore") != "" {
			continue
		}

		// Handle omitempty
		jsonTag := field.Tag.Get("json")
		isOmitEmpty := strings.Contains(jsonTag, "omitempty")

		// Skip nil pointers, slices, and maps
		if (fieldValue.Kind() == reflect.Ptr || fieldValue.Kind() == reflect.Slice || fieldValue.Kind() == reflect.Map) && fieldValue.IsNil() {
			continue
		}

		// Skip zero values if omitempty is set
		if isOmitEmpty && isZeroValue(fieldValue) {
			continue
		}

		// Get field name priority: locationName > json > field name
		name := field.Name
		if locName := field.Tag.Get("locationName"); locName != "" {
			name = locName
		} else if jsonName := field.Tag.Get("json"); jsonName != "" {
			name = strings.Split(jsonName, ",")[0]
		}

		// Handle special cases
		var value interface{}
		if fieldValue.Kind() == reflect.Struct {
			nestedResult, err := marshalWithLocationName(fieldValue)
			if err != nil {
				return nil, err
			}
			var nested interface{}
			if err := json.Unmarshal(nestedResult, &nested); err != nil {
				return nil, err
			}
			value = nested
		} else {
			switch val := fieldValue.Interface().(type) {
			case float64:
				if math.IsNaN(val) {
					value = floatNaN
				} else if math.IsInf(val, 1) {
					value = floatInf
				} else if math.IsInf(val, -1) {
					value = floatNegInf
				} else {
					value = val
				}
			default:
				value = val
			}
		}

		result[name] = value
	}

	return json.Marshal(result)
}

func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice:
		return v.Len() == 0
	case reflect.String:
		// For AWS compatibility, don't consider empty strings as zero values
		// This means empty strings won't be omitted even with omitempty
		return false
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}
