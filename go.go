package jsonschema

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
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

func newReference(t reflect.Type) *Schema {
	return &Schema{Ref: "#/$defs/" + t.Name()}
}

func isRefType(t reflect.Type) bool {
	return t.Kind() == reflect.Map || t.Kind() == reflect.Array || t.Kind() == reflect.Slice
}

func fromGoType(t reflect.Type, opts *goTypeOptions) (*Schema, error) {
	schema, defined := &Schema{}, false
	if _, defined = opts.named[t.Name()]; defined {
		return newReference(t), nil
	} else if t.PkgPath() != "" && t.Name() != "" {
		opts.named[t.Name()] = schema
		defined = true
	}

	if t.Kind() == reflect.Ptr {
		nullable := !isRefType(t.Elem())
		underlying, err := fromGoType(t.Elem(), opts)
		if err != nil {
			return nil, err
		}

		*schema = *underlying
		if nullable {
			if underlying.Ref != "" {
				*schema = Schema{OneOf: []Schema{*underlying, {Type: TypeSet{TypeNull}}}}
			} else {
				if !slices.Contains(schema.Type, TypeNull) {
					schema.Type = append(schema.Type, TypeNull)
				}
			}
		}

		if !defined {
			return schema, nil
		}
		return newReference(t), nil
	}

	// if primitive, we can return early because they are predefined.
	if _, ok := m[t.Kind()]; ok {
		*schema = m[t.Kind()]
		if defined {
			return newReference(t), nil
		}
		return schema, nil
	}

	var (
		s   *Schema
		err error
	)

	switch t.Kind() {
	case reflect.Array, reflect.Slice:
		s, err = arrType(t, opts)
	case reflect.Struct:
		s, err = structType(t, opts)
	case reflect.Map:
		s, err = mapType(t, opts)
	default:
		return nil, fmt.Errorf("cannot map Go type: %v", t)
	}

	if err != nil {
		return nil, fmt.Errorf("schema.FromGoType: %w", err)
	}
	*schema = *s
	if defined {
		return newReference(t), nil
	}
	return s, nil

}

func structType(t reflect.Type, opts *goTypeOptions) (*Schema, error) {
	s := &Schema{Type: TypeSet{TypeObject}}
	if t.Name() != "" {
		opts.named[t.Name()] = s
	}

	s.AdditionalProperties = &False

	num := t.NumField()
	s.Properties = make(map[string]Schema, num)
	var embeddedTypes []reflect.StructField
	for i := 0; i < num; i++ {
		field := t.Field(i)
		fieldType := field.Type

		tag := parseJSONTag(field.Tag.Get("json"))
		name := tag.Name()

		// Embedded reference types like maps, slices and arrays are not directly
		// embeddable, so they are handled like non-embedded fields.
		if field.Anonymous && name == "" && !isRefType(fieldType) {
			embeddedTypes = append(embeddedTypes, field)
			continue
		}

		if name == "" {
			name = field.Name
		}
		fs, err := fromGoType(fieldType, opts)
		if err != nil {
			return nil, fmt.Errorf("schema.FromGoType: %w", err)
		}

		s.Properties[name] = *fs

		if !tag.Contains("omitempty") {
			s.Required = append(s.Required, name)
		}
	}

	for _, field := range embeddedTypes {
		// Only defined types are embeddable, so we know we will get a
		// reference or oneOf if it's a reference
		fieldType := field.Type
		isPtr := fieldType.Kind() == reflect.Ptr
		if isPtr {
			fieldType = fieldType.Elem()
		}

		fs, err := fromGoType(fieldType, opts)
		if err != nil {
			return nil, fmt.Errorf("schema.FromGoType: %w", err)
		}

		embedded := opts.named[fs.Ref[8:]]

		var required, optional []string
		for name, schema := range embedded.Properties {
			if _, ok := s.Properties[name]; ok {
				continue
			}
			s.Properties[name] = schema

			isRequired := slices.Contains(embedded.Required, name)
			if !isRequired {
				optional = append(optional, name)
			} else {
				required = append(required, name)
			}

			if slices.Contains(embedded.Required, name) && !isPtr {
				s.Required = append(s.Required, name)
			}
		}

		if isPtr && len(optional) > 0 && len(optional) > 0 {
			s.DependentRequired = buildDependentRequired(required, optional)
		}
	}

	if t.Name() != "" {
		return &Schema{Ref: "#/$defs/" + t.Name()}, nil
	}
	return s, nil
}

func buildDependentRequired(required, options []string) map[string][]string {
	result := make(map[string][]string)
	for i, key := range required {
		for j, value := range required {
			if i != j { // Avoid adding self-references
				result[key] = append(result[key], value)
			}
		}
	}
	for _, key := range options {
		result[key] = append([]string{}, required...)
	}
	return result
}

func arrType(t reflect.Type, opts *goTypeOptions) (*Schema, error) {
	s := newTyped(TypeArray, true)
	if t.Kind() == reflect.Array {
		s.MaxItems = ptr(t.Len())
	}

	var err error
	if s.Items, err = fromGoType(t.Elem(), opts); err != nil {
		return nil, fmt.Errorf("failed to generate schema for array element type (%s): %w", t.Elem(), err)
	}
	return s, nil
}

func mapType(t reflect.Type, opts *goTypeOptions) (*Schema, error) {
	keyType, valType := t.Key(), t.Elem()
	if keyType.Kind() == reflect.String {
		s := Schema{Type: TypeSet{TypeObject, TypeNull}}
		var err error
		if s.AdditionalProperties, err = fromGoType(valType, opts); err != nil {
			return nil, fmt.Errorf("failed to generate schema for map value type (%s): %w", keyType.Kind(), err)
		}
		return &s, nil
	}

	ks, err := fromGoType(keyType, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate schema for map key type (%s): %w", keyType.Kind(), err)
	}
	vs, err := fromGoType(valType, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate schema for map value type (%s): %w", keyType.Kind(), err)
	}

	s := &Schema{
		Type: TypeSet{TypeObject, TypeNull},
		Properties: map[string]Schema{
			"keys": {
				Type:        TypeSet{TypeArray},
				Items:       ks,
				UniqueItems: ptr(true),
			},
			"values": {
				Type:  TypeSet{TypeArray},
				Items: vs,
			},
		},
		Required:             []string{"keys", "values"},
		AdditionalProperties: &False,
	}
	return s, nil
}

func ptr[T any](v T) *T {
	return &v
}
