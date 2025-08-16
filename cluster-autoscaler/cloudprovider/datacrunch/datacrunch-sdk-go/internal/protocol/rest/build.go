package rest

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/dcerr"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/request"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/internal/logger"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/internal/protocol"
)

const (
	floatNaN    = "NaN"
	floatInf    = "Infinity"
	floatNegInf = "-Infinity"
)

// Whether the byte value can be sent without escaping in Datacrunch URLs
var noEscape [256]bool

var errValueNotSet = fmt.Errorf("value not set")

var byteSliceType = reflect.TypeOf([]byte{})

func init() {
	for i := 0; i < len(noEscape); i++ {
		// Datacrunch expects every character except these to be escaped
		noEscape[i] = (i >= 'A' && i <= 'Z') ||
			(i >= 'a' && i <= 'z') ||
			(i >= '0' && i <= '9') ||
			i == '-' ||
			i == '.' ||
			i == '_' ||
			i == '~'
	}
}

// BuildHandler is a named request handler for building rest protocol requests
var BuildHandler = request.NamedHandler{Name: "rest.Build", Fn: Build}

// Build builds the REST component of a service request.
func Build(r *request.Request) {
	logger.Debug("rest.Build called with ParamsFilled=%v", r.ParamsFilled())
	if r.ParamsFilled() {
		// check if r.Params is a pointer
		var v reflect.Value
		if reflect.TypeOf(r.Params).Kind() != reflect.Ptr {
			v = reflect.ValueOf(r.Params)
		} else {
			v = reflect.ValueOf(r.Params).Elem()
		}
		logger.Debug("rest.Build: building location elements for type %T", r.Params)
		buildLocationElements(r, v, false)
		buildBody(r, v)
	}
}

// BuildAsGET builds the REST component of a service request with the ability to hoist
// data from the body.
func BuildAsGET(r *request.Request) {
	logger.Debug("rest.BuildAsGET called with ParamsFilled=%v", r.ParamsFilled())
	if r.ParamsFilled() {
		// check if r.Params is a pointer
		var v reflect.Value
		if reflect.TypeOf(r.Params).Kind() != reflect.Ptr {
			v = reflect.ValueOf(r.Params)
		} else {
			v = reflect.ValueOf(r.Params).Elem()
		}
		logger.Debug("rest.BuildAsGET: building location elements for type %T", r.Params)
		buildLocationElements(r, v, true)
		buildBody(r, v)
	}
}

func buildLocationElements(r *request.Request, v reflect.Value, buildGETQuery bool) {
	query := r.HTTPRequest.URL.Query()

	// Setup the raw path to match the base path pattern. This is needed
	// so that when the path is mutated a custom escaped version can be
	// stored in RawPath that will be used by the Go client.
	r.HTTPRequest.URL.RawPath = r.HTTPRequest.URL.Path

	logger.Debug("buildLocationElements: struct type=%s, numFields=%d", v.Type().Name(), v.NumField())

	for i := 0; i < v.NumField(); i++ {
		m := v.Field(i)
		fieldName := v.Type().Field(i).Name
		if n := fieldName; n[0:1] == strings.ToLower(n[0:1]) {
			logger.Debug("buildLocationElements: skipping unexported field %s", n)
			continue
		}

		if m.IsValid() {
			field := v.Type().Field(i)
			name := field.Tag.Get("locationName")
			if name == "" {
				name = field.Name
			}
			if kind := m.Kind(); kind == reflect.Ptr {
				m = m.Elem()
			} else if kind == reflect.Interface {
				if !m.Elem().IsValid() {
					logger.Debug("buildLocationElements: skipping invalid interface field %s", field.Name)
					continue
				}
			}
			if !m.IsValid() {
				logger.Debug("buildLocationElements: skipping invalid field %s", field.Name)
				continue
			}
			if field.Tag.Get("ignore") != "" {
				logger.Debug("buildLocationElements: skipping ignored field %s", field.Name)
				continue
			}

			var err error
			switch field.Tag.Get("location") {
			case "headers": // header maps
				logger.Debug("buildLocationElements: building header map for field %s", field.Name)
				err = buildHeaderMap(&r.HTTPRequest.Header, m, field.Tag)
			case "header":
				logger.Debug("buildLocationElements: building header for field %s", field.Name)
				err = buildHeader(&r.HTTPRequest.Header, m, name, field.Tag)
			case "uri":
				logger.Debug("buildLocationElements: building uri for field %s", field.Name)
				err = buildURI(r.HTTPRequest.URL, m, name, field.Tag)
			case "querystring":
				logger.Debug("buildLocationElements: building querystring for field %s", field.Name)
				err = buildQueryString(query, m, name, field.Tag)
			default:
				if buildGETQuery {
					logger.Debug("buildLocationElements: building GET querystring for field %s", field.Name)
					err = buildQueryString(query, m, name, field.Tag)
				}
			}
			r.Error = err
		}
		if r.Error != nil {
			logger.Debug("buildLocationElements: error encountered: %v", r.Error)
			return
		}
	}

	r.HTTPRequest.URL.RawQuery = query.Encode()
	logger.Debug("buildLocationElements: final RawQuery=%s", r.HTTPRequest.URL.RawQuery)
}

func buildBody(r *request.Request, v reflect.Value) {
	params := v.Interface()
	if params == nil {
		logger.Debug("buildBody: params is nil, skipping body build")
		return
	}

	// Check if there's a payload member to use instead of the full params
	if payloadMember := PayloadMember(params); payloadMember != nil {
		logger.Debug("buildBody: using payload member instead of full params")
		params = payloadMember
	}

	switch body := params.(type) {
	case io.ReadSeeker:
		logger.Debug("buildBody: using io.ReadSeeker for body")
		r.SetReaderBody(body)
	case []byte:
		logger.Debug("buildBody: using []byte for body")
		r.SetBufferBody(body)
	case string:
		logger.Debug("buildBody: using string for body")
		r.SetStringBody(body)
	default:
		// JSON marshal everything else
		logger.Debug("buildBody: marshaling params to JSON for body")
		if data, err := json.Marshal(params); err == nil {
			r.SetBufferBody(data)
			r.HTTPRequest.Header.Set("Content-Type", "application/json")
			logger.Debug("buildBody: set Content-Type to application/json")
		} else {
			logger.Debug("buildBody: failed to encode REST request: %v", err)
			r.Error = dcerr.New(request.ErrCodeSerialization,
				"failed to encode REST request", err)
		}
	}
}

func buildHeader(header *http.Header, v reflect.Value, name string, tag reflect.StructTag) error {
	str, err := convertType(v, tag)
	if err == errValueNotSet {
		logger.Debug("buildHeader: value not set for header %s", name)
		return nil
	} else if err != nil {
		logger.Debug("buildHeader: failed to encode REST request for header %s: %v", name, err)
		return dcerr.New(request.ErrCodeSerialization, "failed to encode REST request", err)
	}

	name = strings.TrimSpace(name)
	str = strings.TrimSpace(str)

	logger.Debug("buildHeader: adding header %s: %s", name, str)
	header.Add(name, str)

	return nil
}

func buildHeaderMap(header *http.Header, v reflect.Value, tag reflect.StructTag) error {
	prefix := tag.Get("locationName")
	for _, key := range v.MapKeys() {
		str, err := convertType(v.MapIndex(key), tag)
		if err == errValueNotSet {
			logger.Debug("buildHeaderMap: value not set for key %v", key)
			continue
		} else if err != nil {
			logger.Debug("buildHeaderMap: failed to encode REST request for key %v: %v", key, err)
			return dcerr.New(request.ErrCodeSerialization, "failed to encode REST request", err)

		}
		keyStr := strings.TrimSpace(key.String())
		str = strings.TrimSpace(str)

		logger.Debug("buildHeaderMap: adding header %s%s: %s", prefix, keyStr, str)
		header.Add(prefix+keyStr, str)
	}
	return nil
}

func buildURI(u *url.URL, v reflect.Value, name string, tag reflect.StructTag) error {
	value, err := convertType(v, tag)
	if err == errValueNotSet {
		logger.Debug("buildURI: value not set for uri param %s", name)
		return nil
	} else if err != nil {
		logger.Debug("buildURI: failed to encode REST request for uri param %s: %v", name, err)
		return dcerr.New(request.ErrCodeSerialization, "failed to encode REST request", err)
	}

	logger.Debug("buildURI: replacing {%s} in path with %s", name, EscapePath(value))
	u.Path = strings.ReplaceAll(u.Path, "{"+name+"}", EscapePath(value))

	u.RawPath = strings.ReplaceAll(u.RawPath, "{"+name+"}", EscapePath(value))

	return nil
}

func buildQueryString(query url.Values, v reflect.Value, name string, tag reflect.StructTag) error {
	switch value := v.Interface().(type) {
	case []*string:
		logger.Debug("buildQueryString: adding []*string for query param %s", name)
		for _, item := range value {
			query.Add(name, *item)
		}
	case map[string]*string:
		logger.Debug("buildQueryString: adding map[string]*string for query param %s", name)
		for key, item := range value {
			query.Add(key, *item)
		}
	case map[string][]*string:
		logger.Debug("buildQueryString: adding map[string][]*string for query param %s", name)
		for key, items := range value {
			for _, item := range items {
				query.Add(key, *item)
			}
		}
	default:
		str, err := convertType(v, tag)
		if err == errValueNotSet {
			logger.Debug("buildQueryString: value not set for query param %s", name)
			return nil
		} else if err != nil {
			logger.Debug("buildQueryString: failed to encode REST request for query param %s: %v", name, err)
			return dcerr.New(request.ErrCodeSerialization, "failed to encode REST request", err)
		}
		logger.Debug("buildQueryString: setting query param %s: %s", name, str)
		query.Set(name, str)
	}

	return nil
}

func EscapePath(path string) string {
	return url.PathEscape(path)
}

func convertType(v reflect.Value, tag reflect.StructTag) (str string, err error) {
	v = reflect.Indirect(v)
	if !v.IsValid() {
		logger.Debug("convertType: value is not valid")
		return "", errValueNotSet
	}

	switch value := v.Interface().(type) {
	case string:
		// if the value is a string and the tag has suppressedJSONValue=true and location=header, then encode the value to base64
		if tag.Get("suppressedJSONValue") == "true" && tag.Get("location") == "header" {
			logger.Debug("convertType: encoding string to base64 for header")
			value = base64.StdEncoding.EncodeToString([]byte(value))
		}
		str = value
	case []*string:
		if tag.Get("location") != "header" || tag.Get("enum") == "" {
			logger.Debug("convertType: []*string only supported with location header and enum shapes")
			return "", fmt.Errorf("%T is only supported with location header and enum shapes", value)
		}
		if len(value) == 0 {
			logger.Debug("convertType: []*string is empty")
			return "", errValueNotSet
		}

		buff := &bytes.Buffer{}
		for i, sv := range value {
			if sv == nil || len(*sv) == 0 {
				continue
			}
			if i != 0 {
				buff.WriteRune(',')
			}
			item := *sv
			if strings.Contains(item, `,`) || strings.Contains(item, `"`) {
				item = strconv.Quote(item)
			}
			buff.WriteString(item)
		}
		str = buff.String()
	case []byte:
		logger.Debug("convertType: encoding []byte to base64")
		str = base64.StdEncoding.EncodeToString(value)
	case bool:
		str = strconv.FormatBool(value)
	case int:
		str = strconv.FormatInt(int64(value), 10)
	case int64:
		str = strconv.FormatInt(value, 10)
	case float64:
		switch {
		case math.IsNaN(value):
			str = floatNaN
		case math.IsInf(value, 1):
			str = floatInf
		case math.IsInf(value, -1):
			str = floatNegInf
		default:
			str = strconv.FormatFloat(value, 'f', -1, 64)
		}
	case time.Time:
		format := tag.Get("timestampFormat")
		if len(format) == 0 {
			format = "2006-01-02T15:04:05Z" // default to ISO8601
		}
		str = value.Format(format)
	case map[string]interface{}:
		if len(value) == 0 {
			logger.Debug("convertType: map[string]interface{} is empty")
			return "", errValueNotSet
		}
		escaping := protocol.NoEscape
		if tag.Get("location") == "header" {
			escaping = protocol.Base64Escape
		}
		logger.Debug("convertType: encoding map[string]interface{} to JSON with escaping=%v", escaping)
		str, err = protocol.EncodeJSONValue(value, escaping)
		if err != nil {
			logger.Debug("convertType: unable to encode JSONValue: %v", err)
			return "", fmt.Errorf("unable to encode JSONValue, %v", err)
		}
	default:
		logger.Debug("convertType: unsupported value for param %v (%s)", v.Interface(), v.Type())
		err := fmt.Errorf("unsupported value for param %v (%s)", v.Interface(), v.Type())
		return "", err
	}

	return str, nil
}
