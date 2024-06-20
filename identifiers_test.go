package jsonschema_test

import (
	"context"
	. "jsonschema"
	"net/url"
	"reflect"
	"slices"
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
			EnclosingResourceURIs: []string{
				"https://example.com/root.json#/$defs/B",
			},
		},
		"/$defs/C": {
			BaseURI:                 "urn:uuid:ee564b8a-7a87-4125-8c96-e9f123d6766f",
			CanonResourcePointerURI: "urn:uuid:ee564b8a-7a87-4125-8c96-e9f123d6766f#",
			EnclosingResourceURIs: []string{
				"https://example.com/root.json#/$defs/C",
			},
		},
		"/$defs/B/$defs/X": {
			BaseURI:                 "https://example.com/other.json",
			CanonResourcePlainURI:   "https://example.com/other.json#bar",
			CanonResourcePointerURI: "https://example.com/other.json#/$defs/X",
			EnclosingResourceURIs: []string{
				"https://example.com/root.json#/$defs/B/$defs/X",
			},
		},
		"/$defs/B/$defs/Y": {
			BaseURI:                 "https://example.com/t/inner.json",
			CanonResourcePlainURI:   "https://example.com/t/inner.json#bar",
			CanonResourcePointerURI: "https://example.com/t/inner.json#",
			EnclosingResourceURIs: []string{
				"https://example.com/other.json#/$defs/Y",
				"https://example.com/root.json#/$defs/B/$defs/Y",
			},
		},
		"/$defs/B/$defs/Z": {
			BaseURI:                 "https://example.com/z.json",
			CanonResourcePointerURI: "https://example.com/z.json#",
			EnclosingResourceURIs: []string{
				"https://example.com/other.json#/$defs/Z",
				"https://example.com/root.json#/$defs/B/$defs/Z",
			},
		},
		"/$defs/B/$defs/Z/allOf/0": {
			BaseURI:                 "https://example.com/z.json",
			CanonResourcePlainURI:   "https://example.com/z.json#foo",
			CanonResourcePointerURI: "https://example.com/z.json#/allOf/0",
			EnclosingResourceURIs: []string{
				"https://example.com/other.json#/$defs/Z/allOf/0",
				"https://example.com/root.json#/$defs/B/$defs/Z/allOf/0",
			},
		},
		"/$defs/B/$defs/Z/allOf/0/$defs/_": {
			BaseURI:                 "https://example.com/z.json",
			CanonResourcePlainURI:   "https://example.com/z.json#bar",
			CanonResourcePointerURI: "https://example.com/z.json#/allOf/0/$defs/_",
			EnclosingResourceURIs: []string{
				"https://example.com/root.json#/$defs/B/$defs/Z/allOf/0/$defs/_",
				"https://example.com/other.json#/$defs/Z/allOf/0/$defs/_",
			},
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

		slices.Sort(n.EnclosingResourceURIs)
		slices.Sort(testData.EnclosingResourceURIs)

		if !ok {
			t.Errorf("%s not found", p)
		} else if !reflect.DeepEqual(n, testData) {
			t.Errorf("%s: need %+v", p, testData)
			t.Errorf("%s: have %+v", p, n)
		}
	}
}
