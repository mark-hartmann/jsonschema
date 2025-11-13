package jsonschema_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	. "jsonschema"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestWalk(t *testing.T) {
	loader := NewEmbeddedLoader(testdataFS)

	uri, _ := url.Parse("file:///testdata/miscellaneous-examples/conditional-validation-if-else.schema.json")
	schema, _ := loader.Load(nil, uri)

	ctx := context.Background()

	var l1 []struct{}
	err := Walk(ctx, schema, func(ctx context.Context, _ Scope, _ *Schema) error {
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

	err = Walk(ctx, schema, func(ctx context.Context, scope Scope, s1 *Schema) error {
		if scope.Pointer != "/" {
			l2 = append(l2, scope.Pointer)
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

	err = Walk(ctx, schema, func(ctx context.Context, scope Scope, _ *Schema) error {
		if scope.Pointer != "/" {
			l3 = append(l3, scope.Pointer)
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
	if e := Walk(ctx, &Schema{AllOf: []Schema{
		{Properties: map[string]Schema{"foo": {}}},
	}}, func(ctx context.Context, scope Scope, _ *Schema) error {
		s = scope.Pointer
		return nil
	}); e != nil {
		t.Logf("expected no error, got %v", err)
		t.FailNow()
	}

	if s != "/allOf/0/properties/foo" {
		t.Logf("expected %v, got %v", "/allOf/0/properties/foo", s)
		t.FailNow()
	}

	err = Walk(ctx, &False, func(_ context.Context, _ Scope, _ *Schema) error {
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
	filterWalkFunc := func(fn WalkFunc, filter func(kw string, s *Schema) bool) WalkFunc {
		return func(ctx context.Context, scope Scope, schema *Schema) error {
			if scope.Pointer == "/" {
				return fn(ctx, scope, schema)
			}
			segments := strings.Split(scope.Pointer, "/")
			keyword := segments[len(segments)-1]
			switch keyword {
			case "not", "if", "then", "else", "items", "contains", "additionalProperties", "propertyNames":
			default:
				keyword = segments[len(segments)-2]
			}

			if filter(keyword, schema) {
				return fn(ctx, scope, schema)
			}
			return Skip
		}
	}

	filterTestSchema := Schema{
		AllOf: []Schema{{}, {Not: &Schema{}}, {}},
		Not:   &Schema{Description: "foo"},
	}

	filterFunc := func(kw string, s *Schema) bool {
		return kw == "allOf" && s.IsTrue()
	}
	_ = Walk(ctx, &filterTestSchema, filterWalkFunc(func(ctx context.Context, scope Scope, schema *Schema) error {
		if scope.Pointer != "/" {
			l4 = append(l4, scope.Pointer)
		}
		return nil
	}, filterFunc))

	slices.Sort(l4)
	slices.Sort(l4c)
	if !slices.Equal(l4, l4c) {
		t.Logf("expected %v, got %v", l4c, l4)
		t.FailNow()
	}

	ptrTest := Schema{
		AllOf: []Schema{
			{},
		},
		Defs: map[string]Schema{
			"foo": {},
			"bar": {},
		},
		Items: &Schema{},
	}

	for i, cause := range []string{
		"/items",
		"/allOf/0",
		"/$defs/foo",
	} {
		err = Walk(ctx, &ptrTest, func(ctx context.Context, scope Scope, schema *Schema) error {
			if scope.Pointer == cause {
				return errors.New("unexpected error")
			}
			return nil
		})

		if err == nil {
			t.Errorf("expected error at test %d, got nil", i)
		}
	}
}

func TestWalk_Modifying(t *testing.T) {
	ptrTest := Schema{
		Defs: map[string]Schema{
			"foo": {},
			"bar": {},
		},
		AllOf: []Schema{
			{},
			{},
		},
		AdditionalProperties: &False,
	}

	ptrTest2 := Schema{
		Defs: map[string]Schema{
			"foo": {Comment: "replaced"},
			"bar": {},
		},
		AllOf: []Schema{
			{Comment: "modified"},
			{},
		},
		AdditionalProperties: &Schema{
			Comment: "replaced",
			Type:    TypeSet{TypeArray},
			Items: &Schema{
				Type: TypeSet{TypeNumber},
			},
		},
	}

	ctx := context.Background()

	_ = Walk(ctx, &ptrTest, func(ctx context.Context, scope Scope, schema *Schema) error {
		if scope.Pointer == "/$defs/foo" {
			*schema = Schema{Comment: "replaced"}
		} else if scope.Pointer == "/allOf/0" {
			schema.Comment = "modified"
		} else if scope.Pointer == "/additionalProperties" {
			*schema = Schema{
				Comment: "replaced",
				Type:    TypeSet{TypeArray},
				Items:   &Schema{Type: TypeSet{TypeInteger}},
			}
		} else if scope.Pointer == "/additionalProperties/items" {
			*schema = Schema{
				Type: TypeSet{TypeNumber},
			}
		}
		return nil
	})

	if !reflect.DeepEqual(ptrTest, ptrTest2) {
		t.Errorf("\nhave: %s\nneed: %s", &ptrTest, &ptrTest2)
		t.FailNow()
	}
}

func ExampleWalk() {
	const p = `
{
  "$ref": "#/$defs/len",
  "minItems": 1,
  "$defs": {
    "len": {
      "minItems": 2
    }
  }
}`
	s := Schema{}
	_ = json.Unmarshal([]byte(p), &s)

	ctx := context.Background()
	err := Walk(ctx, &s, func(ctx context.Context, scope Scope, s *Schema) error {
		if s.Ref != "" {
			s2, err := ResolveReference(ResolveConfig{}, s.Ref, s)
			if err != nil {
				return fmt.Errorf("failed to resolve reference %q: %w", s.Ref, err)
			}

			// The new s is walked after this function returns, applying
			// this function to both schemas in the slice. We remove the
			// reference pointer to prevent endless cycles ((/allOf/0)+)
			s.Ref = ""
			*s = Schema{AllOf: []Schema{*s, *s2}}
		}
		return nil
	})

	fmt.Println(s.String(), err)
	// Output: {"allOf":[{"$defs":{"len":{"minItems":2}},"minItems":1},{"minItems":2}]} <nil>
}
