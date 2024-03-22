package jsonschema

import (
	"encoding/json"
	"reflect"
)

// Copy creates a deep copy of a schema.
func Copy(src Schema) Schema {
	return Schema{
		Schema:               src.Schema,
		Vocabulary:           copyMap(src.Vocabulary),
		ID:                   src.ID,
		Ref:                  src.Ref,
		DynamicRef:           src.DynamicRef,
		Defs:                 copyMap(src.Defs, Copy),
		Comment:              src.Comment,
		AllOf:                copySlice(src.AllOf, Copy),
		AnyOf:                copySlice(src.AnyOf, Copy),
		OneOf:                copySlice(src.OneOf, Copy),
		Not:                  copyPtr(src.Not, Copy),
		If:                   copyPtr(src.If, Copy),
		Then:                 copyPtr(src.Then, Copy),
		Else:                 copyPtr(src.Else, Copy),
		DependentSchemas:     copyMap(src.DependentSchemas, Copy),
		PrefixItems:          copySlice(src.PrefixItems, Copy),
		Items:                copyPtr(src.Items, Copy),
		Contains:             copyPtr(src.Contains, Copy),
		Properties:           copyMap(src.Properties, Copy),
		PatternProperties:    copyMap(src.PatternProperties, Copy),
		AdditionalProperties: copyPtr(src.AdditionalProperties, Copy),
		PropertyNames:        copyPtr(src.PropertyNames, Copy),
		Type:                 copySlice(src.Type),
		Enum:                 copyAny(src.Enum),
		Const:                copyAny(src.Const),
		MultipleOf:           copyPtr(src.MultipleOf),
		Maximum:              copyPtr(src.Maximum),
		ExclusiveMaximum:     copyPtr(src.ExclusiveMaximum),
		Minimum:              copyPtr(src.Minimum),
		ExclusiveMinimum:     copyPtr(src.ExclusiveMinimum),
		MaxLength:            copyPtr(src.MaxLength),
		MinLength:            copyPtr(src.MinLength),
		Pattern:              copyPtr(src.Pattern),
		MaxItems:             copyPtr(src.MaxItems),
		MinItems:             copyPtr(src.MinItems),
		UniqueItems:          copyPtr(src.UniqueItems),
		MaxContains:          copyPtr(src.MaxContains),
		MinContains:          copyPtr(src.MinContains),
		MaxProperties:        copyPtr(src.MaxProperties),
		MinProperties:        copyPtr(src.MinProperties),
		Required:             copySlice(src.Required),
		DependentRequired: copyMap(src.DependentRequired, func(src []string) []string {
			return copySlice(src)
		}),
		Title:       src.Title,
		Description: src.Description,
		Default:     copyAny(src.Default),
		Deprecated:  copyPtr(src.Deprecated),
		ReadOnly:    copyPtr(src.ReadOnly),
		WriteOnly:   copyPtr(src.WriteOnly),
		Examples:    copyAny(src.Examples),
	}
}

// copyAny copies any data by marshalling and unmarshalling it. This is a somewhat
// costly operation, but the only way to reliably copy unknown types without massive
// amounts of reflection magic. This method is only to be used for the following
// fields guaranteed to be json compliant data types:
//   - Schema.Enum
//   - Schema.Const
//   - Schema.Examples
//   - Schema.Default
func copyAny[T any](v T) T {
	var c T
	rv := reflect.ValueOf(v)
	if !rv.IsValid() || rv.IsZero() {
		return c
	}
	d, _ := json.Marshal(v)
	_ = json.Unmarshal(d, &c)
	return c
}

func copyPtr[T any](src *T, fn ...func(T) T) *T {
	if src == nil {
		return nil
	}

	var c T
	if len(fn) > 0 {
		for i := range fn {
			c = fn[i](*src)
		}
	} else {
		c = *src
	}
	return &c
}

func copySlice[T any](src []T, fn ...func(T) T) []T {
	if src == nil {
		return nil
	}

	var c = make([]T, len(src), len(src))
	for i := 0; i < len(src); i++ {
		if len(fn) > 0 {
			for j := range fn {
				c[i] = fn[j](src[i])
			}
		} else {
			c[i] = src[i]
		}
	}
	return c
}

func copyMap[K comparable, V any](src map[K]V, fn ...func(V) V) map[K]V {
	if src == nil {
		return nil
	}

	c := make(map[K]V)
	for key, value := range src {
		if len(fn) > 0 {
			for j := range fn {
				c[key] = fn[j](value)
			}
		} else {
			c[key] = value
		}
	}
	return c
}
