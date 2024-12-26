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
	type Bar struct {
		Prop string `json:"prop1"`
	}

	type Foo struct {
		Bar
		Bar2 Bar
	}

	type FooBar struct {
		Bar
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
		//"embedded struct": {
		//	In: Foo{},
		//	Out: &Schema{
		//		Ref: "#/$defs/Foo",
		//		Defs: map[string]Schema{
		//			"Foo": {
		//				Type: TypeSet{TypeObject},
		//				Properties: map[string]Schema{
		//					"Bar":  {Ref: "#/$defs/Bar"},
		//					"Bar2": {Ref: "#/$defs/Bar"},
		//				},
		//				Required:             []string{"Bar", "Bar2"},
		//				AdditionalProperties: &False,
		//			},
		//			"Bar": {
		//				Type: TypeSet{TypeObject},
		//				Properties: map[string]Schema{
		//					"prop1": {Type: TypeSet{TypeString}},
		//				},
		//				Required:             []string{"prop1"},
		//				AdditionalProperties: &False,
		//			},
		//		},
		//	},
		//},
		//"recursive embedded struct": {
		//	In: FooBar{},
		//	Out: &Schema{
		//		Ref: "#/$defs/FooBar",
		//		Defs: map[string]Schema{
		//			"FooBar": {
		//				Type: TypeSet{TypeObject},
		//				Properties: map[string]Schema{
		//					"FooBar": {Ref: "#/$defs/Foo6Bar"},
		//				},
		//				AdditionalProperties: &False,
		//			},
		//		},
		//	},
		//},
		//"non-root recursive embedded struct": {
		//	In:  map[string][]FooBar{},
		//	Out: &Schema{},
		//},
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
