package jsonschema_test

import (
	"encoding/json"
	. "jsonschema"
	"math"
	"reflect"
	"strconv"
	"testing"
)

// TestFromGoType_Primitives covers the basic, primitive types.
func TestFromGoType_Primitives(t *testing.T) {
	var (
		uint8min = json.Number(strconv.FormatUint(0, 10))
		uint8max = json.Number(strconv.FormatUint(math.MaxUint8, 10))
		int16min = json.Number(strconv.FormatInt(math.MinInt16, 10))
		int16max = json.Number(strconv.FormatInt(math.MaxInt16, 10))
		intMin   = json.Number(strconv.FormatInt(math.MinInt, 10))
		intMax   = json.Number(strconv.FormatInt(math.MaxInt, 10))
	)

	type IntType int
	type StrManyPtr ***string

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
		{
			In: IntType(0), Out: &Schema{
				Ref: "#/$defs/IntType",
				Defs: map[string]Schema{
					"IntType": {
						Type:    TypeSet{TypeInteger},
						Minimum: &intMin,
						Maximum: &intMax,
					},
				},
			},
		},
		{
			In: StrManyPtr(ptr(ptr(ptr("")))), Out: &Schema{
				Ref: "#/$defs/StrManyPtr",
				Defs: map[string]Schema{
					"StrManyPtr": {
						Type: TypeSet{TypeString, TypeNull},
					},
				},
			},
		},
	}

	for _, test := range tests {
		s, e := FromGoType(reflect.TypeOf(test.In))
		if e != nil {
			t.Errorf("unexpected error: %e", e)
			continue
		}

		if !reflect.DeepEqual(s, test.Out) {
			t.Errorf("\nhave %s\nneed %s", s, test.Out)
		}
	}
}

func TestFromGoType_Embedded(t *testing.T) {
	type Address struct {
		Street string `json:"street"`
		City   string `json:"town"`

		Others []Address `json:"others"`
	}

	type User struct {
		Name string `json:"name"`
		City string `json:"city"`
		Address
	}

	type Foo struct {
		A string
	}

	type Bar struct {
		B string
		Foo
	}

	type EmbeddedOmitEmpty struct {
		B string `json:"b,omitempty"`
		C string `json:"c"`
		D string `json:"d,omitempty"`
		E string `json:"e,omitempty"`
		F string `json:"f"`
	}

	type MapType map[string]string

	tests := map[string]struct {
		In  any
		Out *Schema
	}{
		"embedded map no json tag": {
			In: struct {
				MapType
			}{},
			Out: &Schema{
				Type: TypeSet{TypeObject},
				Properties: map[string]Schema{
					"MapType": {Ref: "#/$defs/MapType"},
				},
				Required:             []string{"MapType"},
				AdditionalProperties: &False,
				Defs: map[string]Schema{
					"MapType": {
						Type:                 TypeSet{TypeObject, TypeNull},
						AdditionalProperties: &Schema{Type: TypeSet{TypeString}},
					},
				},
			},
		},
		"embedded map omitempty": {
			In: struct {
				MapType `json:",omitempty"`
			}{},
			Out: &Schema{
				Type: TypeSet{TypeObject},
				Properties: map[string]Schema{
					"MapType": {Ref: "#/$defs/MapType"},
				},
				AdditionalProperties: &False,
				Defs: map[string]Schema{
					"MapType": {
						Type:                 TypeSet{TypeObject, TypeNull},
						AdditionalProperties: &Schema{Type: TypeSet{TypeString}},
					},
				},
			},
		},
		"nested embedded structs": {
			In: struct {
				C string
				Bar
			}{},
			Out: &Schema{
				Type: TypeSet{TypeObject},
				Properties: map[string]Schema{
					"A": {Type: TypeSet{TypeString}},
					"B": {Type: TypeSet{TypeString}},
					"C": {Type: TypeSet{TypeString}},
				},
				AdditionalProperties: &False,
				Required:             []string{"C", "B", "A"},
			},
		},
		"field priority": {
			In: struct {
				A bool
				Foo
			}{},
			Out: &Schema{
				Type: TypeSet{TypeObject},
				Properties: map[string]Schema{
					"A": {Type: TypeSet{TypeBoolean}},
				},
				AdditionalProperties: &False,
				Required:             []string{"A"},
			},
		},
		"embedded struct ptr with omitempty fields": {
			In: struct {
				A string `json:"a"`
				*EmbeddedOmitEmpty
			}{},
			Out: &Schema{
				Type: TypeSet{TypeObject},
				Properties: map[string]Schema{
					"a": {Type: TypeSet{TypeString}},
					"b": {Type: TypeSet{TypeString}},
					"c": {Type: TypeSet{TypeString}},
					"d": {Type: TypeSet{TypeString}},
					"e": {Type: TypeSet{TypeString}},
					"f": {Type: TypeSet{TypeString}},
				},
				AdditionalProperties: &False,
				Required:             []string{"a"},
				DependentRequired: map[string][]string{
					"b": {"c", "f"},
					"c": {"f"},
					"d": {"c", "f"},
					"e": {"c", "f"},
					"f": {"c"},
				},
			},
		},
		"embedded struct overlap with custom json name": {
			In: User{},
			Out: &Schema{
				Ref: "#/$defs/User",
				Defs: map[string]Schema{
					"User": {
						Type: TypeSet{TypeObject},
						Properties: map[string]Schema{
							"name":   {Type: TypeSet{TypeString}},
							"street": {Type: TypeSet{TypeString}},
							"city":   {Type: TypeSet{TypeString}},
							"town":   {Type: TypeSet{TypeString}},
							"others": {
								Type: TypeSet{TypeArray, TypeNull},
								Items: &Schema{
									Ref: "#/$defs/Address",
								},
							},
						},
						Required:             []string{"name", "city", "street", "town", "others"},
						AdditionalProperties: &False,
					},
					"Address": {
						Type: TypeSet{TypeObject},
						Properties: map[string]Schema{
							"street": {Type: TypeSet{TypeString}},
							"town":   {Type: TypeSet{TypeString}},
							"others": {
								Type: TypeSet{TypeArray, TypeNull},
								Items: &Schema{
									Ref: "#/$defs/Address",
								},
							},
						},
						AdditionalProperties: &False,
						Required:             []string{"street", "town", "others"},
					},
				},
			},
		},
		"embedded struct ptr": {
			In: struct {
				*Foo
			}{},
			Out: &Schema{
				Type: TypeSet{TypeObject},
				Properties: map[string]Schema{
					"A": {Type: TypeSet{TypeString}},
				},
				AdditionalProperties: &False,
			},
		},
		"embedded struct json name": {
			In: struct {
				Foo `json:"foo"`
			}{},
			Out: &Schema{
				Type: TypeSet{TypeObject},
				Properties: map[string]Schema{
					"foo": {Ref: "#/$defs/Foo"},
				},
				AdditionalProperties: &False,
				Required:             []string{"foo"},
				Defs: map[string]Schema{
					"Foo": {
						Type: TypeSet{TypeObject},
						Properties: map[string]Schema{
							"A": {Type: TypeSet{TypeString}},
						},
						AdditionalProperties: &False,
						Required:             []string{"A"},
					},
				},
			},
		},
		"embedded struct ptr json name": {
			In: struct {
				*Foo `json:"foo"`
			}{},
			Out: &Schema{
				Type: TypeSet{TypeObject},
				Properties: map[string]Schema{
					"foo": {OneOf: []Schema{
						{Ref: "#/$defs/Foo"},
						{Type: TypeSet{TypeNull}},
					}},
				},
				AdditionalProperties: &False,
				Required:             []string{"foo"},
				Defs: map[string]Schema{
					"Foo": {
						Type: TypeSet{TypeObject},
						Properties: map[string]Schema{
							"A": {Type: TypeSet{TypeString}},
						},
						AdditionalProperties: &False,
						Required:             []string{"A"},
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

// TestFromGoType_Struct covers all struct-related cases, including anonymous
// structs, defined structs and embedded types.
func TestFromGoType_Struct(t *testing.T) {
	type Comment struct {
		Text    string    `json:"text"`
		Replies []Comment `json:"replies,omitempty"`
	}

	type Node struct {
		Data []byte `json:"data"`
		Next *Node  `json:"next"`
	}

	type MapType map[string]string

	type boolean bool

	tests := map[string]struct {
		In  any
		Out *Schema
	}{
		"anonymous struct": {
			In: struct {
				Foo string   `json:"foo"`
				Bar string   `json:"bar,omitempty"`
				Baz *string  `json:"baz"`
				Qux **string `json:"qux,omitempty"`
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
		"pointer to anonymous struct": {
			In: &struct {
				Value string `json:"value"`
			}{},
			Out: &Schema{
				Type: TypeSet{TypeObject, TypeNull},
				Properties: map[string]Schema{
					"value": {Type: TypeSet{TypeString}},
				},
				AdditionalProperties: &False,
				Required:             []string{"value"},
			},
		},
		"nested anonymous structs": {
			In: struct {
				Foo struct {
					A string `json:"a"`
				} `json:"foo"`
				Bar struct {
					A string `json:"a"`
				} `json:"bar,omitempty"`
				Baz *struct {
					A *string
				} `json:"baz"`
				Quz *struct {
					A *string `json:",omitempty"`
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
							"A": {Type: TypeSet{TypeString, TypeNull}},
						},
						AdditionalProperties: &False,
						Required:             []string{"A"},
					},
					"qux": {
						Type: TypeSet{TypeObject, TypeNull},
						Properties: map[string]Schema{
							"A": {Type: TypeSet{TypeString, TypeNull}},
						},
						AdditionalProperties: &False,
					},
				},
				AdditionalProperties: &False,
				Required:             []string{"foo", "baz"},
			},
		},
		"anonymous struct with defined map field": {
			In: struct {
				Map MapType
			}{},
			Out: &Schema{
				Type: TypeSet{TypeObject},
				Properties: map[string]Schema{
					"Map": {Ref: "#/$defs/MapType"},
				},
				Defs: map[string]Schema{
					"MapType": {
						Type:                 TypeSet{TypeObject, TypeNull},
						AdditionalProperties: &Schema{Type: TypeSet{TypeString}},
					},
				},
				AdditionalProperties: &False,
				Required:             []string{"Map"},
			},
		},
		"defined struct": {
			In: Comment{},
			Out: &Schema{
				Ref: "#/$defs/Comment",
				Defs: map[string]Schema{
					"Comment": {
						Type: TypeSet{TypeObject},
						Properties: map[string]Schema{
							"text": {Type: TypeSet{TypeString}},
							"replies": {
								Type: TypeSet{TypeArray, TypeNull},
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
		"pointer to defined struct": {
			In: &Comment{},
			Out: &Schema{
				OneOf: []Schema{
					{Ref: "#/$defs/Comment"},
					{Type: TypeSet{TypeNull}},
				},
				Defs: map[string]Schema{
					"Comment": {
						Type: TypeSet{TypeObject},
						Properties: map[string]Schema{
							"text": {Type: TypeSet{TypeString}},
							"replies": {
								Type: TypeSet{TypeArray, TypeNull},
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
		"recursive struct": {
			In: Node{},
			Out: &Schema{
				Ref: "#/$defs/Node",
				Defs: map[string]Schema{
					"Node": {
						Type: TypeSet{TypeObject},
						Properties: map[string]Schema{
							"data": {
								Type: TypeSet{TypeArray, TypeNull},
								Items: &Schema{
									Type:    TypeSet{TypeInteger},
									Minimum: ptr(json.Number("0")),
									Maximum: ptr(json.Number("255")),
								},
							},
							"next": {
								OneOf: []Schema{
									{Ref: "#/$defs/Node"},
									{Type: TypeSet{TypeNull}},
								},
							},
						},
						AdditionalProperties: &False,
						Required:             []string{"data", "next"},
					},
				},
			},
		},
		"ignore": {
			In: struct {
				A string `json:"-,omitempty"`
				B string `json:"-"`
			}{},
			Out: &Schema{
				Type: TypeSet{TypeObject},
				Properties: map[string]Schema{
					"-": {Type: TypeSet{TypeString}},
				},
				AdditionalProperties: &False,
			},
		},
		"nullable object without properties": {
			In: &struct {
				A string `json:"-"`
			}{},
			Out: &Schema{
				Type:                 TypeSet{TypeObject, TypeNull},
				AdditionalProperties: &False,
			},
		},
		"quoted": {
			In: struct {
				A bool    `json:",string"`
				B boolean `json:",string"`
				C uint16  `json:",string"`
				D int8    `json:",string"`
				E float32 `json:",string"`
			}{},
			Out: &Schema{
				Type: TypeSet{TypeObject},
				Properties: map[string]Schema{
					"A": {
						Enum: []any{"true", "false"},
					},
					"B": {
						Enum: []any{"true", "false"},
					},
					"C": {
						Type:    TypeSet{TypeString},
						Pattern: ptr(`^(0|[1-9]\d*)$`),
					},
					"D": {
						Type:    TypeSet{TypeString},
						Pattern: ptr(`^-?(0|[1-9]\d*)$`),
					},
					"E": {
						Type:    TypeSet{TypeString},
						Pattern: ptr(`^-?(0|[1-9]\d*)(\.\d+)?$`),
					},
				},
				AdditionalProperties: &False,
				Required:             []string{"A", "B", "C", "D", "E"},
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

// TestFromGoType covers slices, arrays and maps.
func TestFromGoType(t *testing.T) {
	var (
		uint8min = json.Number(strconv.FormatUint(0, 10))
		uint8max = json.Number(strconv.FormatUint(math.MaxUint8, 10))
		intMin   = json.Number(strconv.FormatInt(math.MinInt, 10))
		intMax   = json.Number(strconv.FormatInt(math.MaxInt, 10))
	)

	type MapType map[string]string
	type SliceType []string
	type ArrayType [1024]string
	type MapTypePtr *map[string]string

	var mtPtr MapTypePtr

	tests := map[string]struct {
		In  any
		Out *Schema
	}{
		"slice of strings": {
			In: []string{},
			Out: &Schema{
				Type: TypeSet{TypeArray, TypeNull},
				Items: &Schema{
					Type: TypeSet{TypeString},
				},
			},
		},
		"array of uint8 pointers": {
			In: [9]*uint8{},
			Out: &Schema{
				Type: TypeSet{TypeArray, TypeNull},
				Items: &Schema{
					Type:    TypeSet{TypeInteger, TypeNull},
					Minimum: &uint8min,
					Maximum: &uint8max,
				},
				MaxItems: ptr(9),
			},
		},
		"defined slice of strings": {
			In: SliceType{},
			Out: &Schema{
				Ref: "#/$defs/SliceType",
				Defs: map[string]Schema{
					"SliceType": {
						Type: TypeSet{TypeArray, TypeNull},
						Items: &Schema{
							Type: TypeSet{TypeString},
						},
					},
				},
			},
		},
		"defined array of strings": {
			In: ArrayType{},
			Out: &Schema{
				Ref: "#/$defs/ArrayType",
				Defs: map[string]Schema{
					"ArrayType": {
						Type: TypeSet{TypeArray, TypeNull},
						Items: &Schema{
							Type: TypeSet{TypeString},
						},
						MaxItems: ptr(1024),
					},
				},
			},
		},
		"map with string keys and uint8 values": {
			In: map[string]uint8{},
			Out: &Schema{
				Type: TypeSet{TypeObject, TypeNull},
				AdditionalProperties: &Schema{
					Type:    TypeSet{TypeInteger},
					Minimum: &uint8min,
					Maximum: &uint8max,
				},
			},
		},
		"map with int keys and string values": {
			In: map[int]string{},
			Out: &Schema{
				Type: TypeSet{TypeObject, TypeNull},
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
			},
		},
		"defined string map": {
			In: MapType{},
			Out: &Schema{
				Ref: "#/$defs/MapType",
				Defs: map[string]Schema{
					"MapType": {
						Type:                 TypeSet{TypeObject, TypeNull},
						AdditionalProperties: &Schema{Type: TypeSet{TypeString}},
					},
				},
			},
		},
		"pointer to defined string map": {
			In: &MapType{},
			Out: &Schema{
				Ref: "#/$defs/MapType",
				Defs: map[string]Schema{
					"MapType": {
						Type:                 TypeSet{TypeObject, TypeNull},
						AdditionalProperties: &Schema{Type: TypeSet{TypeString}},
					},
				},
			},
		},
		"named map ptr": {
			In: mtPtr,
			Out: &Schema{
				Ref: "#/$defs/MapTypePtr",
				Defs: map[string]Schema{
					"MapTypePtr": {
						Type: TypeSet{TypeObject, TypeNull},
						AdditionalProperties: &Schema{
							Type: TypeSet{TypeString},
						},
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
