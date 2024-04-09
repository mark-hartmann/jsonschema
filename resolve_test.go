package jsonschema_test

import (
	"context"
	. "jsonschema"
	"net/url"
	"reflect"
	"testing"
)

func TestComputeIdentifiers(t *testing.T) {
	loader := NewEmbeddedLoader(testdataFS)

	uri, _ := url.Parse("file:///testdata/miscellaneous-examples/schema-id-examples.schema.json")
	schema, _ := loader.Load(context.Background(), uri)

	m, _ := ComputeIdentifiers(*schema)

	tests := map[string]Identifiers{
		"/$defs/A": {
			BaseURI:                 "https://example.com/root.json",
			CanonResourcePlainURI:   "https://example.com/root.json#foo",
			CanonResourcePointerURI: "https://example.com/root.json#/$defs/A",
		},
		"/$defs/B": {
			BaseURI:                 "https://example.com/other.json",
			CanonResourcePointerURI: "https://example.com/other.json#",
		},
		"/$defs/C": {
			BaseURI:                 "urn:uuid:ee564b8a-7a87-4125-8c96-e9f123d6766f",
			CanonResourcePointerURI: "urn:uuid:ee564b8a-7a87-4125-8c96-e9f123d6766f#",
		},
		"/$defs/B/$defs/X": {
			BaseURI:                 "https://example.com/other.json",
			CanonResourcePlainURI:   "https://example.com/other.json#bar",
			CanonResourcePointerURI: "https://example.com/other.json#/$defs/X",
		},
		"/$defs/B/$defs/Y": {
			BaseURI:                 "https://example.com/t/inner.json",
			CanonResourcePlainURI:   "https://example.com/t/inner.json#bar",
			CanonResourcePointerURI: "https://example.com/t/inner.json#",
		},
		"/$defs/B/$defs/Z": {
			BaseURI:                 "https://example.com/z.json",
			CanonResourcePointerURI: "https://example.com/z.json#",
		},
		"/$defs/B/$defs/Z/allOf/0": {
			BaseURI:                 "https://example.com/z.json",
			CanonResourcePlainURI:   "https://example.com/z.json#foo",
			CanonResourcePointerURI: "https://example.com/z.json#/allOf/0",
		},
		"/$defs/B/$defs/Z/allOf/0/$defs/_": {
			BaseURI:                 "https://example.com/z.json",
			CanonResourcePlainURI:   "https://example.com/z.json#bar",
			CanonResourcePointerURI: "https://example.com/z.json#/allOf/0/$defs/_",
		},
	}

	if len(tests) != len(m) {
		t.Errorf("mismatching lengths")
		t.Errorf("need %d", len(tests))
		t.Errorf("have %d", len(m))
		t.FailNow()
	}

	for p, testData := range tests {
		n, ok := m[p]
		if !ok {
			t.Errorf("%s not found", p)
		} else if !reflect.DeepEqual(n, testData) {
			t.Errorf("%s: need %+v", p, testData)
			t.Errorf("%s: have %+v", p, n)
		}
	}
}

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
							{Ref: "file:///testdata/miscellaneous-examples/complex-object.schema.json#/properties/SchemaIDs"},
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
			actual, err := ResolveReference(nil, loader, td.ref, td.in, td.in)

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

}
