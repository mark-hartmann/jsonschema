package jsonschema_test

import (
	. "jsonschema"
	"reflect"
	"testing"
)

func TestResolveReference(t *testing.T) {
	type test struct {
		name, ref string
		in, out   *Schema
	}

	var defsSchema = &Schema{
		Defs: map[string]Schema{
			"foo": {
				Type: TypeSet{TypeBoolean},
			},
			"bar": {
				Type: TypeSet{TypeObject},
				Properties: map[string]Schema{
					"a": {Type: TypeSet{TypeString}},
					"b": {
						AnyOf: []Schema{
							{Ref: "#/$defs/foo"},
							{Ref: "/$defs/null_schema"}, // illegal, $defs not defined
							{Ref: "/"},                  // illegal, infinite loop
							{Ref: "#"},
							{Ref: "#/$defs/bar/properties/a"},
							{Ref: "#/$defs/bar/properties/b/$defs/null_schema/$defs/x"},

							// external
							{Ref: "file:///testdata/file-system/entry-schema.schema.json#/properties/storage/oneOf/0"},
							{Ref: "file:///testdata/miscellaneous-examples/arrays.schema.json#/properties/vegetables"},
							{Ref: "file:///testdata/miscellaneous-examples/complex-object.schema.json#/properties/name"},
						},
						Defs: map[string]Schema{
							"null_schema": {
								Type: TypeSet{TypeNull},
								Defs: map[string]Schema{
									"x": {
										Type: TypeSet{TypeArray},
										Items: &Schema{
											Type: TypeSet{TypeNumber},
										},
									},
								},
							},
						},
					},
				},
			},
			"baz": {
				Ref: "#/$defs/bar",
			},
			"bu/z": {},
			"ba~z": {Type: TypeSet{TypeBoolean}},
		},
	}

	var tests = []test{
		{name: "self abs", ref: "#", in: defsSchema, out: defsSchema},
		{name: "self rel", ref: "/", in: defsSchema, out: defsSchema},
		{name: "#_$defs_foo", ref: "#/$defs/foo", in: defsSchema, out: ptr(defsSchema.Defs["foo"])},
		{name: "_$defs_foo", ref: "/$defs/foo", in: defsSchema, out: ptr(defsSchema.Defs["foo"])},
		{name: "_$defs_bar", ref: "/$defs/bar", in: defsSchema, out: ptr(defsSchema.Defs["bar"])},
		{name: "_$defs_bar_properties_a", ref: "/$defs/bar/properties/a", in: defsSchema, out: ptr(defsSchema.Defs["bar"].Properties["a"])},
		{name: "_$defs_bar_properties_a_1", ref: "/$defs/bar/properties/b", in: defsSchema, out: ptr(defsSchema.Defs["bar"].Properties["b"])},
		{name: "_$defs_bar_properties_a_2", ref: "/$defs/bar/properties/b/$defs/null_schema", in: defsSchema, out: ptr(defsSchema.Defs["bar"].Properties["b"].Defs["null_schema"])},
		{name: "#/$defs/baz/properties/a", ref: "#/$defs/baz/properties/a", in: defsSchema, out: ptr(defsSchema.Defs["bar"].Properties["a"])},
		{name: "_$defs_bar_properties_a_3", ref: "/$defs/bar/properties/b/anyOf/1", in: defsSchema, out: nil},
		{name: "invalid ptr", ref: "/foo/", in: defsSchema, out: nil},
		{name: "escaped slash in ptr", ref: "/$defs/bu~1z", in: defsSchema, out: &Schema{}},
		{name: "escaped tilde in ptr", ref: "/$defs/ba~0z", in: defsSchema, out: &Schema{Type: TypeSet{TypeBoolean}}},

		{name: "external ref", ref: "#/$defs/bar/properties/b/anyOf/6", in: defsSchema, out: &Schema{
			Properties: map[string]Schema{
				"type":   {Enum: []any{"disk"}},
				"device": {Type: []Type{TypeString}, Pattern: ptr("^/dev/[^/]+(/[^/]+)*$")},
			},
			Required:             []string{"type", "device"},
			AdditionalProperties: &False,
		}},

		{name: "external ref", ref: "#/$defs/bar/properties/b/anyOf/7", in: defsSchema, out: &Schema{
			Type: TypeSet{TypeArray},
			Items: &Schema{
				Ref: "#/$defs/veggie",
			},
		}},
		{name: "external ref", ref: "#/$defs/bar/properties/b/anyOf/8", in: defsSchema, out: &Schema{Type: TypeSet{TypeString}}},
	}

	res := func() *Schema { c := Copy(*defsSchema); return &c }()
	loader := NewEmbeddedLoader(testdataFS)

	for _, td := range tests {
		t.Run(td.name, func(t *testing.T) {
			actual, err := ResolveReference(ResolveConfig{Loader: loader}, td.ref, td.in)

			if err != nil && td.out != nil {
				t.Logf("got err:\n %v", err)
				t.Logf("need: %s", td.out)
				t.FailNow()
			}

			if !reflect.DeepEqual(actual, td.out) {
				t.Logf("have: %s", actual)
				t.Logf("need: %s", td.out)
				t.FailNow()
			}

			if !reflect.DeepEqual(res, defsSchema) {
				t.Logf("source schema was modified:\n")
				t.Logf("prev: %s", res)
				t.Logf("curr: %s", defsSchema)
				t.FailNow()
			}
		})
	}

	t.Run("subschemas", func(t *testing.T) {
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

		for i, testData := range tests2 {
			s, err := ResolveReference(ResolveConfig{Loader: loader}, testData.ref, root)
			if err != nil && testData.expected != nil {
				t.Errorf("unexpected error %s, test case at %d (%s)", err, i, testData.ref)
			}

			if !reflect.DeepEqual(s, testData.expected) {
				t.Errorf("unexpected value at %d using $ref %q:\nneed: %s\nhave: %s", i,
					testData.ref, testData.expected, s)
			}
		}
	})
}
