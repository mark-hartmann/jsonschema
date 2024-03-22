package jsonschema_test

import (
	"embed"
	"fmt"
	. "jsonschema"
	"reflect"
	"testing"
)

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

//go:embed testdata/*
var refSchemas embed.FS

func TestResolveReference(t *testing.T) {
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
	loader := NewEmbeddedLoader(refSchemas)

	for _, td := range tests {
		t.Run(td.name, func(t *testing.T) {
			actual, err := ResolveReference(loader, td.ref, td.in, td.in)

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

	// out of context
	rel := defsSchema.Defs["bar"]
	fmt.Println(ResolveReference(loader, "#/$defs/foo", &rel, defsSchema))

}
