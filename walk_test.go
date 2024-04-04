package jsonschema_test

import (
	"errors"
	. "jsonschema"
	"net/url"
	"slices"
	"strings"
	"testing"
)

func TestWalk(t *testing.T) {
	loader := NewEmbeddedLoader(testdataFS)

	uri, _ := url.Parse("file:///testdata/miscellaneous-examples/conditional-validation-if-else.schema.json")
	schema, _ := loader.Load(nil, uri)

	var l1 []struct{}
	err := Walk(*schema, func(_ string, _ Schema) error {
		l1 = append(l1, struct{}{})
		return SkipAll
	})

	if err != nil {
		t.Logf("expected no error, got %v", err)
		t.FailNow()
	}

	if len(l1) != 1 {
		t.Logf("expected 1 item, got %v", len(l1))
		t.FailNow()
	}

	var l2 []string
	l2c := []string{
		"/if",
		"/then",
		"/else",
		"/properties/isMember",
		"/properties/membershipNumber",
	}

	err = Walk(*schema, func(s string, s1 Schema) error {
		if s != "/" {
			l2 = append(l2, s)
			return Skip
		}
		return nil
	})

	if err != nil {
		t.Logf("expected no error, got %v", err)
		t.FailNow()
	}

	slices.Sort(l2)
	slices.Sort(l2c)
	if !slices.Equal(l2, l2c) {
		t.Logf("expected %v, got %v", l2c, l2)
		t.FailNow()
	}

	var l3 []string
	l3c := []string{
		"/properties/isMember",
		"/properties/membershipNumber",
		"/if",
		"/if/properties/isMember",
		"/then",
		"/then/properties/membershipNumber",
		"/else",
		"/else/properties/membershipNumber",
	}

	err = Walk(*schema, func(s string, _ Schema) error {
		if s != "/" {
			l3 = append(l3, s)
		}
		return nil
	})

	if err != nil {
		t.Logf("expected no error, got %v", err)
		t.FailNow()
	}

	slices.Sort(l3)
	slices.Sort(l3c)
	if !slices.Equal(l3, l3c) {
		t.Logf("expected %v, got %v", l3c, l3)
		t.FailNow()
	}

	s := ""
	if e := Walk(Schema{AllOf: []Schema{
		{Properties: map[string]Schema{"foo": {}}},
	}}, func(ptr string, _ Schema) error {
		s = ptr
		return nil
	}); e != nil {
		t.Logf("expected no error, got %v", err)
		t.FailNow()
	}

	if s != "/allOf/0/properties/foo" {
		t.Logf("expected %v, got %v", "/allOf/0/properties/foo", s)
		t.FailNow()
	}

	err = Walk(False, func(_ string, _ Schema) error {
		return errors.New("unexpected error")
	})

	if err != nil && err.Error() != "unexpected error" {
		t.Logf("expected %q, got %q", "unexpected error", err)
	}

	if err == nil {
		t.Logf("expected error, got nil")
		t.FailNow()
	}

	var l4 []string
	l4c := []string{
		"/allOf/0",
		"/allOf/2",
	}

	// Example for a "filtered" WalkFunc.
	filterWalkFunc := func(fn WalkFunc, filter func(kw string, s Schema) bool) WalkFunc {
		return func(ptr string, schema Schema) error {
			if ptr == "/" {
				return fn(ptr, schema)
			}
			segments := strings.Split(ptr, "/")
			keyword := segments[len(segments)-1]
			switch keyword {
			case "not", "if", "then", "else", "items", "contains", "additionalProperties", "propertyNames":
			default:
				keyword = segments[len(segments)-2]
			}

			if filter(keyword, schema) {
				return fn(ptr, schema)
			}
			return Skip
		}
	}

	filterTestSchema := Schema{
		AllOf: []Schema{{}, {Not: &Schema{}}, {}},
		Not:   &Schema{Description: "foo"},
	}

	filterFunc := func(kw string, s Schema) bool {
		return kw == "allOf" && s.IsTrue()
	}
	_ = Walk(filterTestSchema, filterWalkFunc(func(ptr string, schema Schema) error {
		if ptr != "/" {
			l4 = append(l4, ptr)
		}
		return nil
	}, filterFunc))

	slices.Sort(l4)
	slices.Sort(l4c)
	if !slices.Equal(l4, l4c) {
		t.Logf("expected %v, got %v", l4c, l4)
		t.FailNow()
	}
}
