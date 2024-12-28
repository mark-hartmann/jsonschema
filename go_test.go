package jsonschema_test

import (
	"encoding/json"
	. "jsonschema"
	"math"
	"reflect"
	"strconv"
	"testing"
)

func TestFromGoType_Primitives(t *testing.T) {
	var (
		uint8min = json.Number(strconv.FormatUint(0, 10))
		uint8max = json.Number(strconv.FormatUint(math.MaxUint8, 10))
		int16min = json.Number(strconv.FormatInt(math.MinInt16, 10))
		int16max = json.Number(strconv.FormatInt(math.MaxInt16, 10))
	)

	tests := []struct {
		In  any
		Out *Schema
	}{
		{In: "", Out: &Schema{Type: TypeSet{TypeString}}},
		{In: false, Out: &Schema{Type: TypeSet{TypeBoolean}}},
		{In: float32(0), Out: &Schema{Type: TypeSet{TypeNumber}}},
		{In: ptr(float64(0)), Out: &Schema{Type: TypeSet{TypeNumber, TypeNull}}},
		{In: uint8(0), Out: &Schema{Type: TypeSet{TypeInteger}, Minimum: &uint8min, Maximum: &uint8max}},
		{In: ptr(uint8(0)), Out: &Schema{Type: TypeSet{TypeInteger, TypeNull}, Minimum: &uint8min, Maximum: &uint8max}},
		{In: int16(0), Out: &Schema{Type: TypeSet{TypeInteger}, Minimum: &int16min, Maximum: &int16max}},
	}

	for _, test := range tests {
		s, e := FromGoType(reflect.TypeOf(test.In))
		if e != nil {
			t.Errorf("unexpected error: %e", e)
			continue
		}

		if !reflect.DeepEqual(s, test.Out) {
			t.Errorf("is %s, expected %s", s, test.Out)
		}
	}
}

func TestFromGoType(t *testing.T) {
	var (
		uint8min = json.Number(strconv.FormatUint(0, 10))
		uint8max = json.Number(strconv.FormatUint(math.MaxUint8, 10))
		intMin   = json.Number(strconv.FormatInt(math.MinInt, 10))
		intMax   = json.Number(strconv.FormatInt(math.MaxInt, 10))
	)

	type Comment struct {
		Text    string    `json:"text"`
		Replies []Comment `json:"replies,omitempty"`
	}

	tests := map[string]struct {
		In  any
		Out *Schema
	}{
		"slice": {In: []string{}, Out: &Schema{
			Type: TypeSet{TypeArray},
			Items: &Schema{
				Type: TypeSet{TypeString},
			},
		}},
		"array": {In: [9]*uint8{}, Out: &Schema{
			Type: TypeSet{TypeArray},
			Items: &Schema{
				Type:    TypeSet{TypeInteger, TypeNull},
				Minimum: &uint8min,
				Maximum: &uint8max,
			},
			MaxItems: ptr(9),
		}},
		"map": {In: map[string]uint8{}, Out: &Schema{
			Type: TypeSet{TypeObject},
			AdditionalProperties: &Schema{
				Type:    TypeSet{TypeInteger},
				Minimum: &uint8min,
				Maximum: &uint8max,
			},
		}},
		"map non-string key": {In: map[int]string{}, Out: &Schema{
			Type: TypeSet{TypeObject},
			Properties: map[string]Schema{
				"keys": {
					Type: TypeSet{TypeArray},
					Items: &Schema{
						Type:    TypeSet{TypeInteger},
						Minimum: &intMin,
						Maximum: &intMax,
					},
					UniqueItems: ptr(true),
				},
				"values": {
					Type: TypeSet{TypeArray},
					Items: &Schema{
						Type: TypeSet{TypeString},
					},
				},
			},
			Required:             []string{"keys", "values"},
			AdditionalProperties: &False,
		}},
		"anon struct": {
			In: struct {
				Foo string  `json:"foo"`
				Bar string  `json:"bar,omitempty"`
				Baz *string `json:"baz"`
				Qux *string `json:"qux,omitempty"`
			}{},
			Out: &Schema{
				Type: TypeSet{TypeObject},
				Properties: map[string]Schema{
					"foo": {Type: TypeSet{TypeString}},
					"bar": {Type: TypeSet{TypeString}},
					"baz": {Type: TypeSet{TypeString, TypeNull}},
					"qux": {Type: TypeSet{TypeString, TypeNull}},
				},
				AdditionalProperties: &False,
				Required:             []string{"foo", "baz"},
			},
		},
		"anon struct nested": {
			In: struct {
				Foo struct {
					A string `json:"a"`
				} `json:"foo"`
				Bar struct {
					A string `json:"a"`
				} `json:"bar,omitempty"`
				Baz *struct {
					A string `json:"a"`
				} `json:"baz"`
				Quz *struct {
					A string `json:"a"`
				} `json:"qux,omitempty"`
			}{},
			Out: &Schema{
				Type: TypeSet{TypeObject},
				Properties: map[string]Schema{
					"foo": {
						Type: TypeSet{TypeObject},
						Properties: map[string]Schema{
							"a": {Type: TypeSet{TypeString}},
						},
						AdditionalProperties: &False,
						Required:             []string{"a"},
					},
					"bar": {
						Type: TypeSet{TypeObject},
						Properties: map[string]Schema{
							"a": {Type: TypeSet{TypeString}},
						},
						AdditionalProperties: &False,
						Required:             []string{"a"},
					},
					"baz": {
						Type: TypeSet{TypeObject, TypeNull},
						Properties: map[string]Schema{
							"a": {Type: TypeSet{TypeString}},
						},
						AdditionalProperties: &False,
						Required:             []string{"a"},
					},
					"qux": {
						Type: TypeSet{TypeObject, TypeNull},
						Properties: map[string]Schema{
							"a": {Type: TypeSet{TypeString}},
						},
						AdditionalProperties: &False,
						Required:             []string{"a"},
					},
				},
				AdditionalProperties: &False,
				Required:             []string{"foo", "baz"},
			},
		},
		"named struct": {
			In: Comment{},
			Out: &Schema{
				Ref: "#/$defs/Comment",
				Defs: map[string]Schema{
					"Comment": {
						Type: TypeSet{TypeObject},
						Properties: map[string]Schema{
							"text": {Type: TypeSet{TypeString}},
							"replies": {
								Type: TypeSet{TypeArray},
								Items: &Schema{
									Ref: "#/$defs/Comment",
								},
							},
						},
						AdditionalProperties: &False,
						Required:             []string{"text"},
					},
				},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s, e := FromGoType(reflect.TypeOf(test.In))
			if e != nil {
				t.Errorf("unexpected error: %s", e)
				return
			}

			if !reflect.DeepEqual(s, test.Out) {
				t.Errorf("\nhave %s\nneed %s", s, test.Out)
			}
		})
	}
}
