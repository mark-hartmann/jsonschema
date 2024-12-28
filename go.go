package jsonschema

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

var (
	numZero      = json.Number("0")
	numMinInt    = json.Number(strconv.FormatInt(math.MinInt, 10))
	numMaxInt    = json.Number(strconv.FormatInt(math.MaxInt, 10))
	numMinInt8   = json.Number(strconv.FormatInt(math.MinInt8, 10))
	numMaxInt8   = json.Number(strconv.FormatInt(math.MaxInt8, 10))
	numMinInt16  = json.Number(strconv.FormatInt(math.MinInt16, 10))
	numMaxInt16  = json.Number(strconv.FormatInt(math.MaxInt16, 10))
	numMinInt32  = json.Number(strconv.FormatInt(math.MinInt32, 10))
	numMaxInt32  = json.Number(strconv.FormatInt(math.MaxInt32, 10))
	numMinInt64  = json.Number(strconv.FormatInt(math.MinInt64, 10))
	numMaxInt64  = json.Number(strconv.FormatInt(math.MaxInt64, 10))
	numMaxUint   = json.Number(strconv.FormatUint(math.MaxUint, 10))
	numMaxUint8  = json.Number(strconv.FormatUint(math.MaxUint8, 10))
	numMaxUint16 = json.Number(strconv.FormatUint(math.MaxUint16, 10))
	numMaxUint32 = json.Number(strconv.FormatUint(math.MaxUint32, 10))
	numMaxUint64 = json.Number(strconv.FormatUint(math.MaxUint64, 10))
)

var m = map[reflect.Kind]Schema{
	reflect.Bool:    {Type: TypeSet{TypeBoolean}},
	reflect.String:  {Type: TypeSet{TypeString}},
	reflect.Float32: {Type: TypeSet{TypeNumber}},
	reflect.Float64: {Type: TypeSet{TypeNumber}},
	reflect.Int:     {Type: TypeSet{TypeInteger}, Minimum: &numMinInt, Maximum: &numMaxInt},
	reflect.Int8:    {Type: TypeSet{TypeInteger}, Minimum: &numMinInt8, Maximum: &numMaxInt8},
	reflect.Int16:   {Type: TypeSet{TypeInteger}, Minimum: &numMinInt16, Maximum: &numMaxInt16},
	reflect.Int32:   {Type: TypeSet{TypeInteger}, Minimum: &numMinInt32, Maximum: &numMaxInt32},
	reflect.Int64:   {Type: TypeSet{TypeInteger}, Minimum: &numMinInt64, Maximum: &numMaxInt64},
	reflect.Uint:    {Type: TypeSet{TypeInteger}, Minimum: &numZero, Maximum: &numMaxUint},
	reflect.Uint8:   {Type: TypeSet{TypeInteger}, Minimum: &numZero, Maximum: &numMaxUint8},
	reflect.Uint16:  {Type: TypeSet{TypeInteger}, Minimum: &numZero, Maximum: &numMaxUint16},
	reflect.Uint32:  {Type: TypeSet{TypeInteger}, Minimum: &numZero, Maximum: &numMaxUint32},
	reflect.Uint64:  {Type: TypeSet{TypeInteger}, Minimum: &numZero, Maximum: &numMaxUint64},
}

type goTypeOptions struct {
	named map[string]*Schema
}

func FromGoType(t reflect.Type) (*Schema, error) {
	opts := &goTypeOptions{named: make(map[string]*Schema)}
	s, err := fromGoType(t, opts)
	if err != nil {
		return nil, err
	}

	if len(opts.named) != 0 {
		s.Defs = make(map[string]Schema)
		for k, v := range opts.named {
			s.Defs[k] = *v
		}
	}
	return s, nil
}

func newTyped(t Type, nullable bool) *Schema {
	s := Schema{}
	s.Type = TypeSet{t}
	if nullable {
		s.Type = append(s.Type, TypeNull)
	}
	return &s
}

func fromGoType(t reflect.Type, opts *goTypeOptions) (*Schema, error) {
	nullable := false
	if t.Kind() == reflect.Ptr {
		nullable = true
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Bool:
		return newTyped(TypeBoolean, nullable), nil
	case reflect.String:
		return newTyped(TypeString, nullable), nil
	case reflect.Float32, reflect.Float64:
		return newTyped(TypeNumber, nullable), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8,
		reflect.Uint16, reflect.Uint32, reflect.Uint64:
		s := m[t.Kind()]
		if nullable {
			s.Type = append(s.Type, TypeNull)
		}
		return &s, nil
	case reflect.Array, reflect.Slice:
		s := newTyped(TypeArray, nullable)

		if t.Kind() == reflect.Array {
			s.MaxItems = ptr(t.Len())
		}

		var err error
		if s.Items, err = fromGoType(t.Elem(), opts); err != nil {
			return nil, fmt.Errorf("schema.FromGoType: %w", err)
		}
		return s, nil
	case reflect.Struct:
		var defined bool
		if _, defined = opts.named[t.Name()]; defined {
			return &Schema{Ref: "#/$defs/" + t.Name()}, nil
		}

		s := newTyped(TypeObject, nullable)
		if t.Name() != "" {
			opts.named[t.Name()] = s
		}

		s.AdditionalProperties = &False

		num := t.NumField()
		s.Properties = make(map[string]Schema, num)
		for i := 0; i < num; i++ {
			field := t.Field(i)
			if field.Anonymous {
				return nil, fmt.Errorf("embedded struct fields are not yet supported")
			}

			fieldType := field.Type

			var (
				fs  *Schema
				err error
			)
			if recStruct(t, fieldType) {
				fs, err = &Schema{Ref: "#/$defs/" + t.Name()}, nil
			} else {
				fs, err = fromGoType(fieldType, opts)
			}
			if err != nil {
				return nil, fmt.Errorf("schema.FromGoType: %w", err)
			}

			var name string
			jsonTag := field.Tag.Get("json")
			if jsonTag != "" {
				parts := strings.Split(jsonTag, ",")
				if parts[0] == "" {
					name = field.Name
				} else {
					name = parts[0]
				}
			} else {
				name = field.Name
			}

			s.Properties[name] = *fs

			if !strings.Contains(jsonTag, ",omitempty") {
				s.Required = append(s.Required, name)
			}
		}

		if t.Name() != "" {
			return &Schema{Ref: "#/$defs/" + t.Name()}, nil
		}
		return s, nil
	case reflect.Map:
		s := Schema{}
		s.Type = TypeSet{TypeObject}
		if nullable {
			s.Type = append(s.Type, TypeNull)
		}

		keyType, valType := t.Key(), t.Elem()
		if keyType.Kind() != reflect.String {
			ks, err := fromGoType(keyType, opts)
			if err != nil {
				return nil, fmt.Errorf("schema.FromGoType: %w", err)
			}
			vs, err := fromGoType(valType, opts)
			if err != nil {
				return nil, fmt.Errorf("schema.FromGoType: %w", err)
			}

			return newMapSchema(ks, vs), nil
		}

		propertyArchetype, err := fromGoType(valType, opts)
		if err != nil {
			return nil, fmt.Errorf("schema.FromGoType: %w", err)
		}
		s.AdditionalProperties = propertyArchetype

		return &s, nil
	default:
		return nil, fmt.Errorf("cannot map Go type: %v", t)
	}
}

func ptr[T any](v T) *T {
	return &v
}

func recStruct(t, t2 reflect.Type) bool {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t2.Kind() == reflect.Ptr {
		t2 = t2.Elem()
	}
	return t.AssignableTo(t2)
}

func newMapSchema(keyType, valueType *Schema) *Schema {
	return &Schema{
		Type: TypeSet{TypeObject},
		Properties: map[string]Schema{
			"keys": {
				Type:        TypeSet{TypeArray},
				Items:       keyType,
				UniqueItems: ptr(true),
			},
			"values": {
				Type:  TypeSet{TypeArray},
				Items: valueType,
			},
		},
		Required:             []string{"keys", "values"},
		AdditionalProperties: &False,
	}
}
