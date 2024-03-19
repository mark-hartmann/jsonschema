package jsonschema_test

import (
	"embed"
	"errors"
	. "jsonschema"
	"net/url"
	"reflect"
	"testing"
)

//go:embed testdata/*
var testdataFS embed.FS

func TestNewEmbeddedLoader(t *testing.T) {
	loader := NewEmbeddedLoader(testdataFS)

	uri, _ := url.Parse("https://example.com/arrays.schema.json")
	if _, err := loader.Load(uri); !errors.Is(err, UnsupportedURI) {
		t.Logf("expected UnsupportedURI")
		t.FailNow()
	}

	uri, _ = url.Parse("file:///testdata/miscellaneous-examples/arrays.schema.json")
	schema, err := loader.Load(uri)

	if err != nil {
		t.Logf("expected schema, got %s", err)
		t.FailNow()
	}

	expected := &Schema{
		ID:          "file:///testdata/miscellaneous-examples/arrays.schema.json",
		Schema:      "https://json-schema.org/draft/2020-12/schema",
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
				Required: []string{
					"veggieName",
					"veggieLike",
				},
				Properties: map[string]Schema{
					"veggieName": {
						Type:        TypeSet{TypeString},
						Description: "The name of the vegetable.",
					},
					"veggieLike": {
						Type:        TypeSet{TypeBoolean},
						Description: "Do I like this vegetable?",
					},
				},
			},
		},
	}

	if schema == nil || !reflect.DeepEqual(schema, expected) {
		t.Logf("have: %s", schema)
		t.Logf("need: %s", expected)
		t.FailNow()
	}

	uri, _ = url.Parse("file:///testdata/unknown-file.txt")
	if _, err = loader.Load(uri); err == nil {
		t.Logf("expected error, got nil")
		t.FailNow()
	}
}

func ptr[T any](v T) *T {
	return &v
}
