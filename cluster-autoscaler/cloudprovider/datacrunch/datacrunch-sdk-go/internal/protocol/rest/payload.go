package rest

import (
	"reflect"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/internal/logger"
)

// PayloadMember returns the payload field member of i if there is one, or nil.
func PayloadMember(i interface{}) interface{} {
	if i == nil {
		logger.Debug("PayloadMember: input is nil")
		return nil
	}

	var v reflect.Value
	if reflect.TypeOf(i).Kind() != reflect.Ptr {
		v = reflect.ValueOf(i)
	} else {
		v = reflect.ValueOf(i).Elem()
	}
	if !v.IsValid() {
		logger.Debug("PayloadMember: value is not valid")
		return nil
	}
	if field, ok := v.Type().FieldByName("_"); ok {
		logger.Debug("PayloadMember: found anonymous field '_' with tag: %v", field.Tag)
		if payloadName := field.Tag.Get("payload"); payloadName != "" {
			logger.Debug("PayloadMember: payload tag found: %s", payloadName)
			field, ok := v.Type().FieldByName(payloadName)
			if !ok {
				logger.Debug("PayloadMember: field %s not found in struct", payloadName)
				return nil
			}
			// Allow any payload type (string, structure, blob, etc.)
			payloadType := field.Tag.Get("type")
			logger.Debug("PayloadMember: field %s type is: %s", payloadName, payloadType)

			payload := v.FieldByName(payloadName)
			if payload.IsValid() || (payload.Kind() == reflect.Ptr && !payload.IsNil()) {
				logger.Debug("PayloadMember: payload field %s is valid, returning interface", payloadName)
				return payload.Interface()
			} else {
				logger.Debug("PayloadMember: payload field %s is not valid or is nil", payloadName)
			}
		} else {
			logger.Debug("PayloadMember: no payload tag found in anonymous field")
		}
	} else {
		logger.Debug("PayloadMember: no anonymous field '_' found in struct")
	}
	return nil
}

const nopayloadPayloadType = "nopayload"

// PayloadType returns the type of a payload field member of i if there is one,
// or "".
func PayloadType(i interface{}) string {
	v := reflect.ValueOf(i)
	// Dereference all pointer levels
	for v.Kind() == reflect.Ptr && !v.IsNil() {
		logger.Debug("PayloadType: dereferencing pointer")
		v = v.Elem()
	}
	if !v.IsValid() {
		logger.Debug("PayloadType: value is not valid after dereferencing")
		return ""
	}

	// Only check for payload fields if the type is a struct
	if v.Kind() != reflect.Struct {
		logger.Debug("PayloadType: value is not a struct, got kind: %v", v.Kind())
		return ""
	}

	if field, ok := v.Type().FieldByName("_"); ok {
		logger.Debug("PayloadType: found anonymous field '_' with tag: %v", field.Tag)
		if noPayload := field.Tag.Get(nopayloadPayloadType); noPayload != "" {
			logger.Debug("PayloadType: found nopayload tag, returning %s", nopayloadPayloadType)
			return nopayloadPayloadType
		}

		if payloadName := field.Tag.Get("payload"); payloadName != "" {
			logger.Debug("PayloadType: found payload tag: %s", payloadName)
			if member, ok := v.Type().FieldByName(payloadName); ok {
				payloadType := member.Tag.Get("type")
				logger.Debug("PayloadType: found payload field %s with type tag: %s", payloadName, payloadType)
				return payloadType
			} else {
				logger.Debug("PayloadType: payload field %s not found in struct", payloadName)
			}
		} else {
			logger.Debug("PayloadType: no payload tag found in anonymous field")
		}
	} else {
		logger.Debug("PayloadType: no anonymous field '_' found in struct")
	}

	return ""
}
