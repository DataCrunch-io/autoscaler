package jsonutil

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"reflect"
	"strings"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/dcerr"
)

// UnmarshalJSONError unmarshal's the reader's JSON document into the passed in
// type. The value to unmarshal the json document into must be a pointer to the
// type.
func UnmarshalJSONError(v interface{}, stream io.Reader) error {
	var errBuf bytes.Buffer
	body := io.TeeReader(stream, &errBuf)

	err := json.NewDecoder(body).Decode(v)
	if err != nil {
		msg := "failed decoding error message"
		if err == io.EOF {
			msg = "error message missing"
			err = nil
		}
		return dcerr.NewUnmarshalError(err, msg, errBuf.Bytes())
	}

	return nil
}

// UnmarshalJSON reads a stream and unmarshals the results in object v with locationName support.
func UnmarshalJSON(v interface{}, stream io.Reader) error {
	return unmarshalWithLocationName(v, stream, false)
}

// UnmarshalJSONCaseInsensitive reads a stream and unmarshals the result into the
// object v. Ignores casing for structure members.
func UnmarshalJSONCaseInsensitive(v interface{}, stream io.Reader) error {
	return unmarshalWithLocationName(v, stream, true)
}

func unmarshalWithLocationName(v interface{}, stream io.Reader, caseInsensitive bool) error {
	// First, unmarshal into a generic interface to handle both objects and arrays
	var raw interface{}
	decoder := json.NewDecoder(stream)
	decoder.UseNumber()

	if err := decoder.Decode(&raw); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	var converted interface{}
	var presentFields map[string]bool

	// Handle different response types
	switch rawData := raw.(type) {
	case map[string]interface{}:
		// Object response - convert field names and track present fields
		converted, presentFields = convertMapKeys(rawData, reflect.TypeOf(v), caseInsensitive)
	case []interface{}:
		// Array response - convert each element if it's an object
		elementType := getArrayElementType(reflect.TypeOf(v))
		convertedArray := make([]interface{}, len(rawData))
		for i, item := range rawData {
			if itemMap, ok := item.(map[string]interface{}); ok {
				// Convert field names for object elements in array
				convertedItem, _ := convertMapKeys(itemMap, elementType, caseInsensitive)
				convertedArray[i] = convertedItem
			} else {
				// Keep non-object elements as-is
				convertedArray[i] = item
			}
		}
		converted = convertedArray
	default:
		// Primitive values (string, number, boolean) - keep as-is
		converted = raw
	}

	// Marshal back to JSON and use standard unmarshaling
	data, err := json.Marshal(converted)
	if err != nil {
		return err
	}

	// Use standard JSON unmarshaling with proper type conversion
	decoder2 := json.NewDecoder(bytes.NewReader(data))
	decoder2.UseNumber()

	if err := decoder2.Decode(v); err != nil {
		return err
	}

	// Handle field presence for omitempty fields (only for object responses)
	if rawMap, ok := raw.(map[string]interface{}); ok {
		if err := handleFieldPresence(reflect.ValueOf(v), presentFields, caseInsensitive); err != nil {
			return err
		}
		return handleSpecialFloats(reflect.ValueOf(v), rawMap)
	}

	return nil
}

// getArrayElementType gets the element type for array/slice types
func getArrayElementType(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		return t.Elem()
	}
	// If not an array, return the type as-is
	return t
}

// convertMapKeys converts JSON field names back to struct field names and tracks field presence
func convertMapKeys(data map[string]interface{}, structType reflect.Type, caseInsensitive bool) (map[string]interface{}, map[string]bool) {
	if structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}

	if structType.Kind() != reflect.Struct {
		return data, nil
	}

	result := make(map[string]interface{})
	presentFields := make(map[string]bool)

	// Create mapping from locationName/json name to struct info
	type fieldInfo struct {
		goFieldName   string
		jsonFieldName string
	}
	fieldMapping := make(map[string]fieldInfo)
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if field.PkgPath != "" {
			continue // skip unexported fields
		}

		fieldName := field.Name

		// Determine the target JSON field name for standard unmarshaling
		var targetJSONName string
		if jsonName := field.Tag.Get("json"); jsonName != "" {
			targetJSONName = strings.Split(jsonName, ",")[0]
			if targetJSONName == "-" {
				continue // Skip fields marked with json:"-"
			}
		} else {
			targetJSONName = fieldName
		}

		info := fieldInfo{
			goFieldName:   fieldName,
			jsonFieldName: targetJSONName,
		}

		// Check locationName first, then json tag
		if locName := field.Tag.Get("locationName"); locName != "" {
			fieldMapping[locName] = info
			if caseInsensitive {
				fieldMapping[strings.ToLower(locName)] = info
			}
		} else if jsonName := field.Tag.Get("json"); jsonName != "" {
			name := strings.Split(jsonName, ",")[0]
			if name != "-" {
				fieldMapping[name] = info
				if caseInsensitive {
					fieldMapping[strings.ToLower(name)] = info
				}
			}
		} else {
			fieldMapping[fieldName] = info
			if caseInsensitive {
				fieldMapping[strings.ToLower(fieldName)] = info
			}
		}
	}

	// Convert the data using the mapping and track presence
	for jsonKey, jsonValue := range data {
		searchKey := jsonKey
		if caseInsensitive {
			searchKey = strings.ToLower(jsonKey)
		}

		if fieldInfo, found := fieldMapping[searchKey]; found {
			// Mark field as present (using Go field name for presence tracking)
			presentFields[fieldInfo.goFieldName] = true

			// Recursively handle nested structures
			if nestedMap, ok := jsonValue.(map[string]interface{}); ok {
				// Find the field type for recursive processing
				if field, found := structType.FieldByName(fieldInfo.goFieldName); found {
					fieldType := field.Type
					if fieldType.Kind() == reflect.Ptr {
						fieldType = fieldType.Elem()
					}
					convertedNested, _ := convertMapKeys(nestedMap, fieldType, caseInsensitive)
					jsonValue = convertedNested
				}
			}
			// Use JSON field name for the result (for standard JSON unmarshaling)
			result[fieldInfo.jsonFieldName] = jsonValue
		} else {
			// Keep the original key if no mapping found
			result[jsonKey] = jsonValue
		}
	}

	return result, presentFields
}

// handleFieldPresence handles omitempty fields based on their presence in JSON
func handleFieldPresence(v reflect.Value, presentFields map[string]bool, caseInsensitive bool) error {
	if presentFields == nil {
		return nil
	}

	v = reflect.Indirect(v)
	if v.Kind() != reflect.Struct {
		return nil
	}

	vType := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := vType.Field(i)
		fieldValue := v.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		fieldName := field.Name
		jsonTag := field.Tag.Get("json")

		// Only process fields with omitempty tag
		if jsonTag == "" || !strings.Contains(jsonTag, "omitempty") {
			continue
		}

		// Check if field was present in JSON
		wasPresent := presentFields[fieldName]

		// If field was not present and has omitempty, reset to zero value
		if !wasPresent {
			switch fieldValue.Kind() {
			case reflect.String:
				// For string fields with omitempty that are not present,
				// we need to distinguish from empty string that WAS present
				// Since standard unmarshaling already happened, we can't change this
				// without a more complex implementation using custom types
			case reflect.Ptr:
				// For pointer fields, set to nil if not present
				fieldValue.Set(reflect.Zero(fieldValue.Type()))
			case reflect.Slice, reflect.Map:
				// For slices/maps, set to nil if not present
				fieldValue.Set(reflect.Zero(fieldValue.Type()))
			}
		}
	}

	return nil
}

// handleSpecialFloats handles javascript special float values like "NaN", "Infinity"
func handleSpecialFloats(v reflect.Value, rawData map[string]interface{}) error {
	v = reflect.Indirect(v)
	if v.Kind() != reflect.Struct {
		return nil
	}

	vType := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := vType.Field(i)
		fieldValue := v.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		// Get the JSON field name to look up in rawData
		jsonFieldName := field.Name
		if locName := field.Tag.Get("locationName"); locName != "" {
			jsonFieldName = locName
		} else if jsonName := field.Tag.Get("json"); jsonName != "" {
			jsonFieldName = strings.Split(jsonName, ",")[0]
		}

		rawValue, exists := rawData[jsonFieldName]
		if !exists {
			continue
		}

		// Handle special string values for float fields
		if strValue, ok := rawValue.(string); ok {
			switch fieldValue.Kind() {
			case reflect.Float32, reflect.Float64:
				var floatVal float64
				switch {
				case strings.EqualFold(strValue, floatNaN):
					floatVal = math.NaN()
				case strings.EqualFold(strValue, floatInf):
					floatVal = math.Inf(1)
				case strings.EqualFold(strValue, floatNegInf):
					floatVal = math.Inf(-1)
				default:
					continue
				}

				if fieldValue.Kind() == reflect.Float32 {
					fieldValue.SetFloat(floatVal)
				} else {
					fieldValue.SetFloat(floatVal)
				}
			case reflect.Ptr:
				if fieldValue.Type().Elem().Kind() == reflect.Float64 {
					var floatVal float64
					switch {
					case strings.EqualFold(strValue, floatNaN):
						floatVal = math.NaN()
					case strings.EqualFold(strValue, floatInf):
						floatVal = math.Inf(1)
					case strings.EqualFold(strValue, floatNegInf):
						floatVal = math.Inf(-1)
					default:
						continue
					}
					fieldValue.Set(reflect.ValueOf(&floatVal))
				}
			}
		}

		// Handle nested structs recursively
		if fieldValue.Kind() == reflect.Struct {
			if nestedMap, ok := rawValue.(map[string]interface{}); ok {
				handleSpecialFloats(fieldValue, nestedMap)
			}
		} else if fieldValue.Kind() == reflect.Ptr && !fieldValue.IsNil() && fieldValue.Elem().Kind() == reflect.Struct {
			if nestedMap, ok := rawValue.(map[string]interface{}); ok {
				handleSpecialFloats(fieldValue.Elem(), nestedMap)
			}
		}
	}

	return nil
}
