package rest

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/dcerr"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/internal/logger"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/internal/protocol"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/internal/util"
)

// UnmarshalHandler is a named request handler for unmarshaling rest protocol requests
var UnmarshalHandler = request.NamedHandler{Name: "rest.Unmarshal", Fn: Unmarshal}

// UnmarshalMetaHandler is a named request handler for unmarshaling rest protocol request metadata
var UnmarshalMetaHandler = request.NamedHandler{Name: "rest.UnmarshalMeta", Fn: UnmarshalMeta}

// Unmarshal unmarshals the REST component of a response in a REST service.
func Unmarshal(r *request.Request) {
	logger.Debug("Unmarshal: called for request %v", r)
	if r.DataFilled() {
		v := reflect.Indirect(reflect.ValueOf(r.Data))
		logger.Debug("Unmarshal: DataFilled, calling unmarshalBody with value type %v", v.Type())
		if err := unmarshalBody(r, v); err != nil {
			logger.Debug("Unmarshal: error from unmarshalBody: %v", err)
			r.Error = err
		}
	}
}

// UnmarshalMeta unmarshals the REST metadata of a response in a REST service
func UnmarshalMeta(r *request.Request) {
	logger.Debug("UnmarshalMeta: called for request %v", r)
	if r.DataFilled() {
		logger.Debug("UnmarshalMeta: DataFilled, calling UnmarshalResponse")
		if err := UnmarshalResponse(r.HTTPResponse, r.Data, false); err != nil {
			logger.Debug("UnmarshalMeta: error from UnmarshalResponse: %v", err)
			r.Error = err
		}
	}
}

// UnmarshalResponse attempts to unmarshal the REST response headers to
// the data type passed in. The type must be a pointer. An error is returned
// with any error unmarshaling the response into the target datatype.
func UnmarshalResponse(resp *http.Response, data interface{}, lowerCaseHeaderMaps bool) error {
	logger.Debug("UnmarshalResponse: called with data type %T, lowerCaseHeaderMaps=%v", data, lowerCaseHeaderMaps)
	v := reflect.Indirect(reflect.ValueOf(data))
	// Only unmarshal location elements for struct types
	if v.Kind() == reflect.Struct {
		logger.Debug("UnmarshalResponse: data is struct, calling unmarshalLocationElements")
		return unmarshalLocationElements(resp, v, lowerCaseHeaderMaps)
	}
	// For non-struct types (like slices), there are no location elements to unmarshal
	logger.Debug("UnmarshalResponse: data is not struct, skipping location elements")
	return nil
}

func unmarshalBody(r *request.Request, v reflect.Value) error {
	logger.Debug("unmarshalBody: called with value type %v", v.Type())
	if field, ok := v.Type().FieldByName("_"); ok {
		logger.Debug("unmarshalBody: found anonymous field '_' with tag: %v", field.Tag)
		if payloadName := field.Tag.Get("payload"); payloadName != "" {
			logger.Debug("unmarshalBody: found payload tag: %s", payloadName)
			pfield, _ := v.Type().FieldByName(payloadName)
			if ptag := pfield.Tag.Get("type"); ptag != "" && ptag != "structure" {
				logger.Debug("unmarshalBody: payload type is %s", ptag)
				payload := v.FieldByName(payloadName)
				if payload.IsValid() {
					logger.Debug("unmarshalBody: payload field %s is valid, type: %v", payloadName, payload.Type())
					switch payload.Interface().(type) {
					case []byte:
						logger.Debug("unmarshalBody: payload is []byte")
						defer func() {
							if err := r.HTTPResponse.Body.Close(); err != nil {
								logger.Debug("unmarshalBody: failed to close response body: %v", err)
								r.Error = dcerr.New(request.ErrCodeSerialization, "failed to close response body", err)
							}
						}()
						b, err := io.ReadAll(r.HTTPResponse.Body)
						if err != nil {
							logger.Debug("unmarshalBody: failed to decode REST response: %v", err)
							return dcerr.New(request.ErrCodeSerialization, "failed to decode REST response", err)
						}

						payload.Set(reflect.ValueOf(b))
						logger.Debug("unmarshalBody: set payload []byte, length=%d", len(b))

					case *string:
						logger.Debug("unmarshalBody: payload is *string")
						defer func() {
							if err := r.HTTPResponse.Body.Close(); err != nil {
								logger.Debug("unmarshalBody: failed to close response body: %v", err)
								r.Error = dcerr.New(request.ErrCodeSerialization, "failed to close response body", err)
							}
						}()
						b, err := io.ReadAll(r.HTTPResponse.Body)
						if err != nil {
							logger.Debug("unmarshalBody: failed to decode REST response: %v", err)
							return dcerr.New(request.ErrCodeSerialization, "failed to decode REST response", err)
						}

						str := string(b)
						payload.Set(reflect.ValueOf(&str))
						logger.Debug("unmarshalBody: set payload *string, length=%d", len(str))

					default:
						logger.Debug("unmarshalBody: payload is of type %s", payload.Type())
						switch payload.Type().String() {
						case "io.ReadCloser":
							logger.Debug("unmarshalBody: payload is io.ReadCloser, setting directly")
							payload.Set(reflect.ValueOf(r.HTTPResponse.Body))

						case "io.ReadSeeker":
							logger.Debug("unmarshalBody: payload is io.ReadSeeker, reading all and wrapping in NopCloser")
							b, err := io.ReadAll(r.HTTPResponse.Body)
							if err != nil {
								logger.Debug("unmarshalBody: failed to read response body: %v", err)
								return dcerr.New(request.ErrCodeSerialization,
									"failed to read response body", err)
							}
							payload.Set(reflect.ValueOf(io.NopCloser(bytes.NewReader(b))))
							logger.Debug("unmarshalBody: set payload io.ReadSeeker, length=%d", len(b))

						default:
							logger.Debug("unmarshalBody: unknown payload type %s", payload.Type())
							if _, err := io.Copy(io.Discard, r.HTTPResponse.Body); err != nil {
								logger.Debug("unmarshalBody: error discarding body: %v", err)
								_ = err // Suppress unused variable warning
							}
							if err := r.HTTPResponse.Body.Close(); err != nil {
								logger.Debug("unmarshalBody: error closing body: %v", err)
								_ = err // Suppress unused variable warning
							}
							return dcerr.New(request.ErrCodeSerialization,
								"failed to decode REST response",
								fmt.Errorf("unknown payload type %s", payload.Type()))
						}
					}
				} else {
					logger.Debug("unmarshalBody: payload field %s is not valid", payloadName)
				}
			} else {
				logger.Debug("unmarshalBody: payload type is empty or 'structure', skipping")
			}
		} else {
			logger.Debug("unmarshalBody: no payload tag found in anonymous field")
		}
	} else {
		logger.Debug("unmarshalBody: no anonymous field '_' found in struct")
	}

	return nil
}

func unmarshalLocationElements(resp *http.Response, v reflect.Value, lowerCaseHeaderMaps bool) error {
	logger.Debug("unmarshalLocationElements: called for type %v, lowerCaseHeaderMaps=%v", v.Type(), lowerCaseHeaderMaps)
	for i := 0; i < v.NumField(); i++ {
		m, field := v.Field(i), v.Type().Field(i)
		if n := field.Name; n[0:1] == strings.ToLower(n[0:1]) {
			logger.Debug("unmarshalLocationElements: skipping unexported field %s", field.Name)
			continue
		}

		if m.IsValid() {
			name := field.Tag.Get("locationName")
			if name == "" {
				name = field.Name
			}
			logger.Debug("unmarshalLocationElements: processing field %s, location=%s, name=%s", field.Name, field.Tag.Get("location"), name)

			switch field.Tag.Get("location") {
			case "statusCode":
				logger.Debug("unmarshalLocationElements: unmarshaling statusCode for field %s", field.Name)
				unmarshalStatusCode(m, resp.StatusCode)

			case "header":
				logger.Debug("unmarshalLocationElements: unmarshaling header for field %s", field.Name)
				err := unmarshalHeader(m, resp.Header.Get(name), field.Tag)
				if err != nil {
					logger.Debug("unmarshalLocationElements: error unmarshaling header for field %s: %v", field.Name, err)
					return dcerr.New(request.ErrCodeSerialization, "failed to decode REST response", err)
				}

			case "headers":
				prefix := field.Tag.Get("locationName")
				logger.Debug("unmarshalLocationElements: unmarshaling headers map for field %s, prefix=%s", field.Name, prefix)
				err := unmarshalHeaderMap(m, resp.Header, prefix, lowerCaseHeaderMaps)
				if err != nil {
					logger.Debug("unmarshalLocationElements: error unmarshaling headers map for field %s: %v", field.Name, err)
					return dcerr.New(request.ErrCodeSerialization, "failed to decode REST response", err)
				}
			default:
				logger.Debug("unmarshalLocationElements: field %s has no recognized location tag", field.Name)
			}
		} else {
			logger.Debug("unmarshalLocationElements: field %s is not valid", field.Name)
		}
	}

	return nil
}

func unmarshalStatusCode(v reflect.Value, statusCode int) {
	logger.Debug("unmarshalStatusCode: called with statusCode=%d, value type=%v", statusCode, v.Type())
	if !v.IsValid() {
		logger.Debug("unmarshalStatusCode: value is not valid, skipping")
		return
	}

	switch v.Interface().(type) {
	case *int64:
		s := int64(statusCode)
		v.Set(reflect.ValueOf(&s))
		logger.Debug("unmarshalStatusCode: set *int64 to %d", s)
	default:
		logger.Debug("unmarshalStatusCode: unsupported type %v", v.Type())
	}
}

func unmarshalHeaderMap(r reflect.Value, headers http.Header, prefix string, normalize bool) error {
	logger.Debug("unmarshalHeaderMap: called with prefix=%s, normalize=%v, value type=%v", prefix, normalize, r.Type())
	if len(headers) == 0 {
		logger.Debug("unmarshalHeaderMap: headers is empty, nothing to do")
		return nil
	}
	switch r.Interface().(type) {
	case map[string]*string: // we only support string map value types
		out := map[string]*string{}
		for k, v := range headers {
			if util.HasPrefixFold(k, prefix) {
				origK := k
				if normalize {
					k = strings.ToLower(k)
				} else {
					k = http.CanonicalHeaderKey(k)
				}
				logger.Debug("unmarshalHeaderMap: adding header key %s (original %s) with value %s", k, origK, v[0])
				out[k[len(prefix):]] = &v[0]
			}
		}
		if len(out) != 0 {
			r.Set(reflect.ValueOf(out))
			logger.Debug("unmarshalHeaderMap: set map[string]*string with %d entries", len(out))
		} else {
			logger.Debug("unmarshalHeaderMap: no matching headers found for prefix %s", prefix)
		}

	default:
		logger.Debug("unmarshalHeaderMap: unsupported value type %v", r.Type())
	}
	return nil
}

func unmarshalHeader(v reflect.Value, header string, tag reflect.StructTag) error {
	logger.Debug("unmarshalHeader: called with value type=%v, header=%q, tag=%v", v.Type(), header, tag)
	switch tag.Get("type") {
	case "jsonvalue":
		if len(header) == 0 {
			logger.Debug("unmarshalHeader: header is empty for jsonvalue, skipping")
			return nil
		}
	case "blob":
		if len(header) == 0 {
			logger.Debug("unmarshalHeader: header is empty for blob, skipping")
			return nil
		}
	default:
		if !v.IsValid() || (header == "" && (!v.IsNil() && v.Elem().Kind() != reflect.String)) {
			logger.Debug("unmarshalHeader: value is not valid or header is empty and not string, skipping")
			return nil
		}
	}

	switch v.Interface().(type) {
	case *string:
		if tag.Get("suppressedJSONValue") == "true" && tag.Get("location") == "header" {
			logger.Debug("unmarshalHeader: decoding base64 for suppressedJSONValue")
			b, err := base64.StdEncoding.DecodeString(header)
			if err != nil {
				logger.Debug("unmarshalHeader: failed to decode JSONValue: %v", err)
				return fmt.Errorf("failed to decode JSONValue, %v", err)
			}
			header = string(b)
		}
		v.Set(reflect.ValueOf(&header))
		logger.Debug("unmarshalHeader: set *string value")
	case []byte:
		logger.Debug("unmarshalHeader: decoding base64 for []byte")
		b, err := base64.StdEncoding.DecodeString(header)
		if err != nil {
			logger.Debug("unmarshalHeader: failed to decode base64: %v", err)
			return err
		}
		v.Set(reflect.ValueOf(b))
		logger.Debug("unmarshalHeader: set []byte value, length=%d", len(b))
	case *bool:
		logger.Debug("unmarshalHeader: parsing bool from header")
		b, err := strconv.ParseBool(header)
		if err != nil {
			logger.Debug("unmarshalHeader: failed to parse bool: %v", err)
			return err
		}
		v.Set(reflect.ValueOf(&b))
		logger.Debug("unmarshalHeader: set *bool value to %v", b)
	case *int64:
		logger.Debug("unmarshalHeader: parsing int64 from header")
		i, err := strconv.ParseInt(header, 10, 64)
		if err != nil {
			logger.Debug("unmarshalHeader: failed to parse int64: %v", err)
			return err
		}
		v.Set(reflect.ValueOf(&i))
		logger.Debug("unmarshalHeader: set *int64 value to %d", i)
	case *float64:
		logger.Debug("unmarshalHeader: parsing float64 from header")
		var f float64
		switch {
		case strings.EqualFold(header, floatNaN):
			f = math.NaN()
			logger.Debug("unmarshalHeader: header is NaN")
		case strings.EqualFold(header, floatInf):
			f = math.Inf(1)
			logger.Debug("unmarshalHeader: header is +Inf")
		case strings.EqualFold(header, floatNegInf):
			f = math.Inf(-1)
			logger.Debug("unmarshalHeader: header is -Inf")
		default:
			var err error
			f, err = strconv.ParseFloat(header, 64)
			if err != nil {
				logger.Debug("unmarshalHeader: failed to parse float64: %v", err)
				return err
			}
			logger.Debug("unmarshalHeader: parsed float64 value %v", f)
		}
		v.Set(reflect.ValueOf(&f))
		logger.Debug("unmarshalHeader: set *float64 value to %v", f)
	case *time.Time:
		format := "2006-01-02T15:04:05Z" // default to ISO8601
		logger.Debug("unmarshalHeader: parsing time.Time from header with format %s", format)
		t, err := time.Parse(format, header)
		if err != nil {
			logger.Debug("unmarshalHeader: failed to parse time.Time: %v", err)
			return err
		}
		v.Set(reflect.ValueOf(&t))
		logger.Debug("unmarshalHeader: set *time.Time value to %v", t)
	case map[string]interface{}:
		escaping := protocol.NoEscape
		if tag.Get("location") == "header" {
			escaping = protocol.Base64Escape
		}
		logger.Debug("unmarshalHeader: decoding JSONValue with escaping=%v", escaping)
		m, err := protocol.DecodeJSONValue(header, escaping)
		if err != nil {
			logger.Debug("unmarshalHeader: failed to decode JSONValue: %v", err)
			return err
		}
		v.Set(reflect.ValueOf(m))
		logger.Debug("unmarshalHeader: set map[string]interface{} value")
	default:
		err := fmt.Errorf("unsupported value for param %v (%s)", v.Interface(), v.Type())
		logger.Debug("unmarshalHeader: unsupported value type: %v", err)
		return err
	}
	return nil
}
