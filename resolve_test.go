package jsonschema_test

import (
	. "jsonschema"
	"reflect"
	"testing"
)

func TestResolveReference(t *testing.T) {
	const schema = `{
    "$id": "https://example.net/root.json",
    "items": {
        "type": "array",
        "items": { 
			"$ref": "#item",
			"additionalProperties": {
				"type": "string"
			}
		}
    },
    "$defs": {
        "single": {
            "$anchor": "item",
            "type": "object",
            "additionalProperties": { "$ref": "other.json" }
        },
		"feinted-ref": {
			"$ref": "/items/items"
		},
		"absolute-refs": {
			"$id": "abs.json",
			"oneOf": [
				{ "$ref": "file:///testdata/miscellaneous-examples/arrays.schema.json" },
				{ "$ref": "file:///testdata/miscellaneous-examples/arrays.schema.json#/properties/vegetables" }
			]
		},
		"special-cases": {
			"$defs": {
				"fo~o": true,
				"ba/r": true
			}
		},
		"vocabs": {
			"$defs": {
				"unevaluatedItems": true,
				"unevaluatedProperties": true,
				"contentSchema": {
					"$ref": "#/$defs/special-cases"
				}
			}
		}
    }
}`

	root := &Schema{}
	_ = root.UnmarshalJSON([]byte(schema))

	type args struct {
		config   ResolveConfig
		ref      string
		resource *Schema
	}
	tests := []struct {
		name    string
		args    args
		want    *Schema
		wantErr string
	}{
		{
			name: "empty string",
			args: args{ref: "", resource: root},
			want: root,
		},
		{
			name: "self",
			args: args{ref: "#", resource: root},
			want: root,
		},
		{
			name: "self but with forward slash",
			args: args{ref: "#/", resource: root},
			want: root,
		},
		{
			name: "known definition",
			args: args{ref: "#/$defs/single", resource: root},
			want: &Schema{
				Anchor: "item",
				Type:   TypeSet{TypeObject},
				AdditionalProperties: &Schema{
					Ref: "other.json",
				},
			},
		},
		{
			name:    "reference to external reference without loader",
			args:    args{ref: "#/$defs/single/additionalProperties", resource: root},
			wantErr: `failed to resolve {"$ref": "other.json"} at "https://example.net/root.json#/$defs/single/additionalProperties": unable to locate non-embedded resource {"$id": "https://example.net/other.json"}: no loader configured`,
		},
		{
			name: "non-nil subschema",
			args: args{ref: "#/items", resource: root},
			want: &Schema{
				Type: TypeSet{TypeArray},
				Items: &Schema{
					Ref: "#item",
					AdditionalProperties: &Schema{
						Type: TypeSet{TypeString},
					},
				},
			},
		},
		{
			name:    "forward slash equals empty string",
			args:    args{ref: "#/items/", resource: root},
			wantErr: `invalid reference https://example.net/root.json#/items/: invalid segment "": unknown keyword`,
		},
		{
			name:    "nil subschema",
			args:    args{ref: "#/propertyNames", resource: root},
			wantErr: `failed to resolve https://example.net/root.json#/propertyNames: expected non-nil schema at "#/propertyNames"`,
		},
		{
			name: "nested non-nil subschema",
			args: args{ref: "#/items/items/additionalProperties", resource: root},
			want: &Schema{
				Type: TypeSet{TypeString},
			},
		},
		{
			name:    "unknown keyword in the middle of the pointer",
			args:    args{ref: "#/items/unknown/additionalProperties", resource: root},
			wantErr: `invalid reference https://example.net/root.json#/items/unknown/additionalProperties: invalid segment "unknown": unknown keyword`,
		},
		{
			name:    "unknown schema definition followed by keyword",
			args:    args{ref: "#/$defs/unknown/additionalProperties", resource: root},
			wantErr: `failed to resolve https://example.net/root.json#/$defs/unknown/additionalProperties: unknown key "unknown" at "#/$defs"`,
		},
		{
			name:    "feinted reference pointer",
			args:    args{ref: "#/$defs/feinted-ref", resource: root},
			wantErr: `failed to resolve {"$ref": "/items/items"} at "https://example.net/root.json#/$defs/feinted-ref": unable to locate non-embedded resource {"$id": "https://example.net/items/items"}: no loader configured`,
		},
		{
			name:    "missing def name",
			args:    args{ref: "#/$defs", resource: root},
			wantErr: `invalid reference https://example.net/root.json#/$defs: invalid segment "$defs": does not point to schema`,
		},
		{
			name: "absolute uri",
			args: args{ref: "https://example.net/root.json#/$defs/single", resource: root},
			want: &Schema{
				Anchor: "item",
				Type:   TypeSet{TypeObject},
				AdditionalProperties: &Schema{
					Ref: "other.json",
				},
			},
		},
		{
			name: "absolute uri with anchor",
			args: args{ref: "https://example.net/root.json#item", resource: root},
			want: &Schema{
				Anchor: "item",
				Type:   TypeSet{TypeObject},
				AdditionalProperties: &Schema{
					Ref: "other.json",
				},
			},
		},
		{
			name:    "absolute uri with unknown anchor",
			args:    args{ref: "https://example.net/abs.json#item", resource: root},
			wantErr: `unable to locate embedded resource: unknown anchor "item" at "https://example.net/abs.json"`,
		},
		{
			name: "existing subschema in array",
			args: args{
				ref:      "#/$defs/absolute-refs/oneOf/0",
				config:   ResolveConfig{Loader: NewEmbeddedLoader(testdataFS)},
				resource: root,
			},
			want: &Schema{
				Schema:      "https://json-schema.org/draft/2020-12/schema",
				ID:          "file:///testdata/miscellaneous-examples/arrays.schema.json",
				Comment:     "https://json-schema.org/learn/miscellaneous-examples#arrays-of-things",
				Description: "A representation of a person, company, organization, or place",
				Type:        TypeSet{TypeObject},
				Properties: map[string]Schema{
					"fruits": {
						Type: TypeSet{TypeArray},
						Items: &Schema{
							Type: TypeSet{TypeString},
						},
					},
					"vegetables": {
						Type: TypeSet{TypeArray},
						Items: &Schema{
							Ref: "#/$defs/veggie",
						},
					},
				},
				Defs: map[string]Schema{
					"veggie": {
						Type: TypeSet{TypeObject},
						Properties: map[string]Schema{
							"veggieLike": {
								Type:        TypeSet{TypeBoolean},
								Description: "Do I like this vegetable?",
							},
							"veggieName": {
								Type:        TypeSet{TypeString},
								Description: "The name of the vegetable.",
							},
						},
						Required: []string{"veggieName", "veggieLike"},
					},
				},
			},
		},
		{
			name: "existing subschema in array different index",
			args: args{
				ref:      "#/$defs/absolute-refs/oneOf/1",
				config:   ResolveConfig{Loader: NewEmbeddedLoader(testdataFS)},
				resource: root,
			},
			want: &Schema{
				Type: TypeSet{TypeArray},
				Items: &Schema{
					Ref: "#/$defs/veggie",
				},
			},
		},
		{
			name:    "array index out of bounds",
			args:    args{ref: "#/$defs/absolute-refs/oneOf/2", resource: root},
			wantErr: `failed to resolve https://example.net/root.json#/$defs/absolute-refs/oneOf/2: index out of bounds (2/1) at "#/$defs/absolute-refs/oneOf"`,
		},
		{
			// the reference is validated first, so the resource id does not change like
			// previously, so it's root.json and not abs.json
			name:    "invalid array index",
			args:    args{ref: "#/$defs/absolute-refs/oneOf/two", resource: root},
			wantErr: `invalid reference https://example.net/root.json#/$defs/absolute-refs/oneOf/two: invalid segment "two": invalid array index: "two"`,
		},
		{
			name:    "missing array index",
			args:    args{ref: "#/$defs/absolute-refs/oneOf", resource: root},
			wantErr: `invalid reference https://example.net/root.json#/$defs/absolute-refs/oneOf: invalid segment "oneOf": does not point to schema`,
		},
		{
			name:    "unknown keyword",
			args:    args{ref: "#/$defs/absolute-refs/test", resource: root},
			wantErr: `invalid reference https://example.net/root.json#/$defs/absolute-refs/test: invalid segment "test": unknown keyword`,
		},
		{
			name: "escaping tilde",
			args: args{ref: "#/$defs/special-cases/$defs/fo~0o", resource: root},
			want: &Schema{},
		},
		{
			name: "escaping slash",
			args: args{ref: "#/$defs/special-cases/$defs/ba~1r", resource: root},
			want: &Schema{},
		},
		{
			name: "unevaluated items",
			args: args{ref: "#/$defs/vocabs/$defs/unevaluatedItems", resource: root},
			want: &True,
		},
		{
			name: "unevaluated properties",
			args: args{ref: "#/$defs/vocabs/$defs/unevaluatedProperties", resource: root},
			want: &True,
		},
		{
			name: "content schema",
			args: args{ref: "#/$defs/vocabs/$defs/contentSchema", resource: root},
			want: &Schema{
				Defs: map[string]Schema{
					"fo~o": True,
					"ba/r": True,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveReference(tt.args.config, tt.args.ref, tt.args.resource)
			if (err != nil) != (tt.wantErr != "") {
				if err != nil {
					t.Errorf("unexpected error\nhave: %s\nneed: %v", err, nil)
				} else {
					t.Errorf("expected error, got nil\nhave: %v\nneed: %s", nil, tt.wantErr)
				}
				return
			}

			if tt.wantErr != "" && !reflect.DeepEqual(err.Error(), tt.wantErr) {
				t.Errorf("error does not equal expected error\nhave: %v\nneed: %v", err, tt.wantErr)
			} else if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("have does not equal need\nhave: %v\nneed = %v", got, tt.want)
			}
		})
	}
}

func TestResolveReference_Embedded(t *testing.T) {
	const idsSchema = `{
  "$id": "https://example.com/schema.json",
  "$defs": {
    "A": {
      "$anchor": "foo"
    },
    "B": {
      "$id": "other.json",
      "$defs": {
        "X": {
          "$anchor": "bar",
		  "not": {
			"$ref": "#/$defs/Y/oneOf/2"
		  }
        },
		"Y": {
		  "oneOf": [
			{"$ref": "#/$defs/X"},
			{"$ref": "file:///testdata/miscellaneous-examples/arrays.schema.json#/properties/vegetables"},
			{"$ref": "https://domain.tld/schema.json#/not"},
			{"$ref": "/schema.json#/$defs/C"}
		  ]
		}
	  }
	},
	"C": {
	  "type": "string"
	},
	"D": {
      "$id": "https://domain.tld/schema.json",
      "not": {
        "$ref": "https://example.com/other.json#bar"
      }
	}
  }
}
`

	root := &Schema{}
	_ = root.UnmarshalJSON([]byte(idsSchema))

	tests2 := []struct {
		ref      string
		expected *Schema
	}{
		{
			ref:      "#foo",
			expected: &Schema{Anchor: "foo"},
		},
		{
			ref:      "other.json#bar",
			expected: &Schema{Anchor: "bar", Not: &Schema{Ref: "#/$defs/Y/oneOf/2"}},
		},
		{
			ref: "other.json",
			expected: &Schema{
				ID: "other.json",
				Defs: map[string]Schema{
					"X": {
						Anchor: "bar",
						Not:    &Schema{Ref: "#/$defs/Y/oneOf/2"},
					},
					"Y": {
						OneOf: []Schema{
							{Ref: "#/$defs/X"},
							{Ref: "file:///testdata/miscellaneous-examples/arrays.schema.json#/properties/vegetables"},
							{Ref: "https://domain.tld/schema.json#/not"},
							{Ref: "/schema.json#/$defs/C"},
						},
					},
				},
			},
		},
		{
			ref:      "#/$defs/B/$defs/X/not",
			expected: &Schema{Anchor: "bar", Not: &Schema{Ref: "#/$defs/Y/oneOf/2"}},
		},
		{
			ref:      "#/$defs/B/$defs/Y/oneOf/1",
			expected: &Schema{Type: TypeSet{TypeArray}, Items: &Schema{Ref: "#/$defs/veggie"}},
		},
		{
			ref: "https://domain.tld/schema.json",
			expected: &Schema{
				ID: "https://domain.tld/schema.json",
				Not: &Schema{
					Ref: "https://example.com/other.json#bar",
				},
			},
		},
	}

	loader := NewEmbeddedLoader(testdataFS)

	for i, testData := range tests2 {
		s, err := ResolveReference(ResolveConfig{Loader: loader}, testData.ref, root)
		if err != nil && testData.expected != nil {
			t.Errorf("unexpected error in test case %d (%s)\nhave: %s", i, testData.ref, err)
			continue
		}

		if !reflect.DeepEqual(s, testData.expected) {
			t.Errorf("unexpected value at %d using $ref %q:\nneed: %s\nhave: %s", i,
				testData.ref, testData.expected, s)
		}
	}
}
