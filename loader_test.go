package jsonschema_test

import (
	"embed"
	"encoding/json"
	"errors"
	. "jsonschema"
	"net/url"
	"reflect"
	"testing"
)

//go:embed testdata/embedded/*
var schemas embed.FS

func TestNewEmbeddedLoader(t *testing.T) {
	load := NewEmbeddedLoader(schemas)

	uri, _ := url.Parse("https://example.com/foo.json")
	if _, err := load(uri); !errors.Is(err, UnsupportedURI) {
		t.Logf("expected UnsupportedURI")
		t.FailNow()
	}

	uri, _ = url.Parse("file:///testdata/embedded/foo.json")
	schema, err := load(uri)

	if err != nil {
		t.Logf("expected schema, got %s", err)
		t.FailNow()
	}

	//goland:noinspection GoRedundantConversion
	expected := &Schema{
		Type: TypeSet{TypeArray},
		Items: &Schema{
			OneOf: []Schema{
				{Ref: "#/$defs/uint8"},
				{Ref: "file:///testdata/embedded/bar.json#/$defs/negativeOne"},
			},
		},
		Defs: map[string]Schema{
			"uint8": {
				Type:    TypeSet{TypeInteger},
				Minimum: ptr(json.Number("0")),
				Maximum: ptr(json.Number("255")),
			},
		},
	}

	if schema == nil || !reflect.DeepEqual(schema, expected) {
		t.Logf("have: %s", schema)
		t.Logf("need: %s", expected)
		t.FailNow()
	}

	uri, _ = url.Parse("file:///testdata/embedded/baz.txt")
	if _, err = load(uri); err == nil {
		t.Logf("expected error, got nil")
		t.FailNow()
	}

	uri, _ = url.Parse("file:///testdata/embedded/buz.txt")
	if _, err = load(uri); err == nil {
		t.Logf("expected error, got nil")
		t.FailNow()
	}
}

func ptr[T any](v T) *T {
	return &v
}
