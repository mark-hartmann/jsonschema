package jsonschema_test

import (
	"encoding/json"
	"fmt"
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
	type ExtremeExample **[8]***map[ExtremeExample]StrManyPtr

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
		{
			In: ptr(StrManyPtr(ptr(ptr(ptr(""))))), Out: &Schema{
				OneOf: []Schema{
					{Ref: "#/$defs/StrManyPtr"},
					{Type: TypeSet{TypeNull}},
				},
				Defs: map[string]Schema{
					"StrManyPtr": {
						Type: TypeSet{TypeString, TypeNull},
					},
				},
			},
		},
		{
			// quite extreme example with mixed, self referencing type and chained pointers
			In: ptr(ExtremeExample(ptr(&[8]***map[ExtremeExample]StrManyPtr{}))),
			Out: &Schema{
				OneOf: []Schema{
					{Ref: "#/$defs/ExtremeExample"},
					{Type: TypeSet{TypeNull}},
				},
				Defs: map[string]Schema{
					"StrManyPtr": {
						Type: TypeSet{TypeString, TypeNull},
					},
					"ExtremeExample": {
						Type: TypeSet{TypeArray, TypeNull},
						Items: &Schema{
							Type: TypeSet{TypeObject, TypeNull},
							Properties: map[string]Schema{
								"keys": {
									Type: TypeSet{TypeArray},
									Items: &Schema{
										Ref: "#/$defs/ExtremeExample",
									},
									UniqueItems: ptr(true),
								},
								"values": {
									Type:  TypeSet{TypeArray},
									Items: &Schema{Ref: "#/$defs/StrManyPtr"},
								},
							},
							AdditionalProperties: &False,
							Required:             []string{"keys", "values"},
						},
						MaxItems: ptr(8),
					},
				},
			},
		},
	}

	for _, test := range tests {
		s, e := FromGoType(reflect.TypeOf(test.In), GoTypeConfig{})
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
				Bar  string
				Baz  bool `json:"Bar"`
				Qux  bool `json:"Quux"`
				Quux string
			}{},
			Out: &Schema{
				Type: TypeSet{TypeObject},
				Properties: map[string]Schema{
					"A":    {Type: TypeSet{TypeBoolean}},
					"Bar":  {Type: TypeSet{TypeBoolean}},
					"Quux": {Type: TypeSet{TypeBoolean}},
				},
				AdditionalProperties: &False,
				Required:             []string{"A", "Bar", "Quux"},
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
			s, e := FromGoType(reflect.TypeOf(test.In), GoTypeConfig{})

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

type A struct {
	B
}

type B struct {
	Name string
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
		"cyclic embedded": {
			In: struct {
				A
				B
			}{},
			Out: &Schema{
				Type: TypeSet{TypeObject},
				Properties: map[string]Schema{
					"Name": {Type: TypeSet{TypeString}},
				},
				AdditionalProperties: &False,
				Required:             []string{"Name"},
			},
		},
		"ignore": {
			In: struct {
				A string `json:"-,omitempty"`
				B string `json:"-"`
				c string
				boolean
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
				A bool     `json:",string"`
				B boolean  `json:",string"`
				C uint16   `json:",string"`
				D int8     `json:",string"`
				E *float32 `json:",string"`
				F *bool    `json:",string"`
			}{},
			Out: &Schema{
				Type: TypeSet{TypeObject},
				Properties: map[string]Schema{
					"A": {
						Enum: []any{"false", "true"},
					},
					"B": {
						Ref: "#/$defs/boolean",
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
						Type:    TypeSet{TypeString, TypeNull},
						Pattern: ptr(`^-?(0|[1-9]\d*)(\.\d+)?$`),
					},
					"F": {
						OneOf: []Schema{
							{Enum: []any{"false", "true"}},
							{Type: TypeSet{TypeNull}},
						},
					},
				},
				AdditionalProperties: &False,
				Required:             []string{"A", "B", "C", "D", "E", "F"},
				Defs: map[string]Schema{
					"boolean": {
						Type: TypeSet{TypeBoolean},
					},
				},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s, e := FromGoType(reflect.TypeOf(test.In), GoTypeConfig{})
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

	var emptyInterface any

	tests := map[string]struct {
		In  any
		Out *Schema
		Err error
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
				OneOf: []Schema{
					{Ref: "#/$defs/MapType"},
					{Type: TypeSet{TypeNull}},
				},
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
		"empty interface value": {
			In:  emptyInterface,
			Out: &True,
		},
		"empty interface fields": {
			In: struct {
				A any `json:",omitempty"`
			}{},
			Out: &Schema{
				Type: TypeSet{TypeObject},
				Properties: map[string]Schema{
					"A": True,
				},
				AdditionalProperties: &False,
			},
		},
		"channel": {
			In:  make(chan int),
			Err: fmt.Errorf("schema.FromGoType: cannot map Go type chan int"),
		},
		"interface field": {
			In: struct {
				A interface{ Print() }
			}{},
			Err: fmt.Errorf(`schema.FromGoType: field "A": cannot map Go type interface { Print() }`),
		},
		"invalid array type": {
			In:  []chan int{},
			Err: fmt.Errorf(`schema.FromGoType: invalid array element type: cannot map Go type chan int`),
		},
		"invalid map key": {
			In:  map[complex128]string{},
			Err: fmt.Errorf(`schema.FromGoType: invalid map key: cannot map Go type complex128`),
		},
		"invalid map value": {
			In:  map[uint8]complex128{},
			Err: fmt.Errorf(`schema.FromGoType: invalid map value: cannot map Go type complex128`),
		},
		"invalid string map value": {
			In: struct {
				A struct {
					B map[string]chan int
				}
			}{},
			Err: fmt.Errorf(`schema.FromGoType: field "A": field "B": invalid map value: cannot map Go type chan int`),
		},
		"pointer to invalid type": {
			In: struct {
				A ****func()
			}{},
			Err: fmt.Errorf(`schema.FromGoType: field "A": cannot map Go type func()`),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s, e := FromGoType(reflect.TypeOf(test.In), GoTypeConfig{})
			if e != nil && test.Err == nil {
				t.Errorf("unexpected error: %s", e)
				return
			}

			if test.Err != nil && e == nil {
				t.Errorf("expected error, got nil")
				return
			}

			if test.Err != nil && e.Error() != test.Err.Error() {
				t.Errorf("\nhave %s\nneed %s", e, test.Err)
			} else if !reflect.DeepEqual(s, test.Out) {
				t.Errorf("\nhave %s\nneed %s", s, test.Out)
			}
		})
	}
}

func TestNewSimpleTypeRepository(t *testing.T) {
	types := NewSimpleTypeRepository()
	types.Store(reflect.TypeFor[bool](), &Schema{Enum: []any{0, 1}}, true)
	types.Store(reflect.TypeFor[*bool](), &Schema{Enum: []any{0, 1, nil}}, true)

	type BooleanPtr *bool
	types.Store(reflect.TypeFor[BooleanPtr](), &Schema{Enum: []any{"no", "yes", "none"}}, true)

	types.Store(QuotedTypeOf(reflect.TypeFor[uint8]()), &Schema{Enum: []any{"0", "1", "2"}}, true)
	types.Store(QuotedTypeOf(reflect.TypeFor[*uint8]()), &Schema{Enum: []any{"2", "1", "0"}}, true)

	s, err := FromGoType(reflect.TypeFor[struct {
		A bool            `json:",omitempty"`
		B *bool           `json:",omitempty"`
		C **bool          `json:",omitempty"`
		D BooleanPtr      `json:",omitempty"`
		E *BooleanPtr     `json:",omitempty"`
		F []bool          `json:",omitempty"`
		G map[string]bool `json:",omitempty"`
		H uint8           `json:",omitempty"`
		I *uint8          `json:",omitempty"`
		J uint8           `json:",omitempty,string"`
		K *uint8          `json:",omitempty,string"`
	}](), GoTypeConfig{Types: types})

	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	expected := &Schema{
		Type: TypeSet{TypeObject},
		Properties: map[string]Schema{
			"A": {Enum: []any{0, 1}},
			"B": {
				OneOf: []Schema{
					{Enum: []any{0, 1}},
					{Type: TypeSet{TypeNull}},
				},
			},
			"C": {
				OneOf: []Schema{
					{Enum: []any{0, 1}},
					{Type: TypeSet{TypeNull}},
				},
			},
			"D": {Enum: []any{"no", "yes", "none"}},
			"E": {
				OneOf: []Schema{
					{Enum: []any{"no", "yes", "none"}},
					{Type: TypeSet{TypeNull}},
				},
			},
			"F": {
				Type: TypeSet{TypeArray, TypeNull},
				Items: &Schema{
					Enum: []any{0, 1},
				},
			},
			"G": {
				Type: TypeSet{TypeObject, TypeNull},
				AdditionalProperties: &Schema{
					Enum: []any{0, 1},
				},
			},
			"H": {
				Type:    TypeSet{TypeInteger},
				Minimum: ptr(json.Number("0")),
				Maximum: ptr(json.Number("255")),
			},
			"I": {
				Type:    TypeSet{TypeInteger, TypeNull},
				Minimum: ptr(json.Number("0")),
				Maximum: ptr(json.Number("255")),
			},
			"J": {Enum: []any{"0", "1", "2"}},
			"K": {
				OneOf: []Schema{
					{Enum: []any{"0", "1", "2"}},
					{Type: TypeSet{TypeNull}},
				},
			},
		},
		AdditionalProperties: &False,
	}
	if !reflect.DeepEqual(s, expected) {
		t.Errorf("\nhave %s\nneed %s", s, expected)
	}
}
