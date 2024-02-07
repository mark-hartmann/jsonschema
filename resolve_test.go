package jsonschema_test

import (
	"embed"
	"encoding/json"
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
						{Ref: "file:///testdata/schema/aaa.schema.json#/items/oneOf/0"},
						{Ref: "file:///testdata/schema/aaa.schema.json#/items/oneOf/1"},
						{Ref: "file:///testdata/schema/aaa.schema.json#/items/oneOf/2"},
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

//go:embed testdata/schema/*
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
		{name: "external ref", ref: "#/$defs/bar/properties/b/anyOf/6", in: defsSchema, out: &Schema{Type: TypeSet{TypeInteger}, Minimum: ptr(json.Number("8"))}},
		{name: "external ref", ref: "#/$defs/bar/properties/b/anyOf/7", in: defsSchema, out: &Schema{Type: TypeSet{TypeNull}}},
		{name: "external ref", ref: "#/$defs/bar/properties/b/anyOf/8", in: defsSchema, out: &Schema{Type: TypeSet{TypeArray}}},
	}

	res := &Schema{}
	_ = json.Unmarshal([]byte(defsSchema.String()), res)

	for _, td := range tests {
		t.Run(td.name, func(t *testing.T) {
			loader := NewEmbeddedLoader(refSchemas)
			actual, err := ResolveReference(loader, td.ref, td.in)

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
