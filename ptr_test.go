package jsonschema_test

import (
	. "jsonschema"
	"testing"
)

func TestValidateReferencePointer(t *testing.T) {
	var refTests = map[string]struct {
		ref, err string
	}{
		"empty string":              {ref: ""},
		"rel self":                  {ref: "/"},
		"root":                      {ref: "#/"},
		"anyOf with index":          {ref: "#/anyOf/0"},
		"anyOf without index but /": {ref: "#/anyOf/"},
		"not":                       {ref: "/not"},
		"if then":                   {ref: "/if/then"},
		"if then with slash": {
			ref: "/if/then/",
			err: `invalid segment "": unknown keyword`,
		},
		"properties map key":         {ref: "#/properties/foo"},
		"properties digit map key":   {ref: "#/properties/123"},
		"escaped properties map key": {ref: "/properties/fo~1o"},
		"umlauts":                    {ref: "#/$defs/äöü"},
		"trailing slashes":           {ref: "/if/properties//items"},
		"trailing slashes 2x": {
			ref: "/if/properties///",
			err: `invalid segment "": unknown keyword`,
		},
		"illegal map key": {
			ref: "#/foo",
			err: `invalid segment "foo": unknown keyword`,
		},
		"anyOf without index": {
			ref: "#/anyOf",
			err: `invalid segment "anyOf": does not point to schema`,
		},
		"anyOf zero padded index": {
			ref: "#/anyOf/01",
			err: `invalid segment "01": invalid array index`,
		},
		"anyOf with non digit index": {
			ref: "#/anyOf/one",
			err: `invalid segment "one": invalid array index`,
		},
		"illegal index": {
			ref: "/then/1",
			err: `invalid segment "1": unknown keyword`,
		},
		"illegal map key #2": {
			ref: "/else/foo",
			err: `invalid segment "foo": unknown keyword`,
		},
		"oneOf invalid signed index": {
			ref: "#/oneOf/+123",
			err: `invalid segment "+123": invalid array index`,
		},
		"oneOf invalid signed index #2": {
			ref: "#/oneOf/-123",
			err: `invalid segment "-123": invalid array index`,
		},
		"invalid escape": {
			ref: "/properties/fo~ao",
			err: `invalid segment "fo~ao": invalid escape sequence: ~a`,
		},
		"missing map/object key": {
			ref: "/properties/foo/properties",
			err: `invalid segment "properties": does not point to schema`,
		},
		"illegal escape char": {
			ref: "#/properties/~",
			err: `invalid segment "~": invalid escape sequence: ~`,
		},
		"invalid JSON pointer": {
			ref: "properties/foo",
			err: "invalid JSON pointer: properties/foo",
		},
	}

	for name, data := range refTests {
		t.Run(name, func(t *testing.T) {

			err := ValidateReferencePointer(data.ref)

			if data.err == "" && err != nil {
				t.Logf("expected no error, got %q", err)
				t.FailNow()
			}

			if (data.err != "" && err == nil) || (err != nil && err.Error() != data.err) {
				t.Logf("expected error %q, got %q", data.err, err)
				t.FailNow()
			}
		})
	}
}
