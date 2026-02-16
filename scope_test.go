package jsonschema_test

import (
	"jsonschema"
	"testing"
)

func TestScope_PointerRoot(t *testing.T) {
	schema := &jsonschema.Schema{
		ID: "https://example.com/root.json",
		Defs: map[string]jsonschema.Schema{
			"B": {
				ID: "other.json",
				Defs: map[string]jsonschema.Schema{
					"Y": {
						OneOf: []jsonschema.Schema{
							{
								ID:     "t/inner.json",
								Anchor: "bar",
							},
						},
					},
				},
			},
		},
	}

	s, _ := jsonschema.NewScope[struct{}](schema, nil)
	ns := schema.Defs["B"]
	n, _ := s.Next(&ns, jsonschema.Step{Keyword: "$defs", Key: ptr("B")})
	ns2 := n.Schema.Defs["Y"]
	n2, _ := n.Next(&ns2, jsonschema.Step{Keyword: "$defs", Key: ptr("Y")})
	ns3 := n2.Schema.OneOf[0]
	n3, _ := n2.Next(&ns3, jsonschema.Step{Keyword: "oneOf", Index: ptr(0)})

	tests := map[string]struct {
		scope       *jsonschema.Scope[struct{}]
		baseURI     string
		uri         string
		pointer     string
		pointerRoot string
	}{
		"s1: /": {
			scope:       s,
			baseURI:     "https://example.com/root.json",
			uri:         "https://example.com/root.json",
			pointer:     "/",
			pointerRoot: "/",
		},
		"s2: /$defs/B": {
			scope:       n,
			baseURI:     "https://example.com/other.json",
			uri:         "https://example.com/other.json",
			pointer:     "/",
			pointerRoot: "/$defs/B",
		},
		"s3: /$defs/B/$defs/Y": {
			scope:       n2,
			baseURI:     "https://example.com/other.json",
			uri:         "https://example.com/other.json#/$defs/Y",
			pointer:     "/$defs/Y",
			pointerRoot: "/$defs/B/$defs/Y",
		},
		"s4: /$defs/B/$defs/Y/oneOf/0": {
			scope:       n3,
			baseURI:     "https://example.com/t/inner.json",
			uri:         "https://example.com/t/inner.json",
			pointer:     "/",
			pointerRoot: "/$defs/B/$defs/Y/oneOf/0",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			scope := test.scope
			if scope.PointerRoot() != test.pointerRoot {
				t.Errorf("PointerRoot: have %q, need %q", s.PointerRoot(), test.pointerRoot)
			}
			if scope.Pointer() != test.pointer {
				t.Errorf("Pointer: have %q, need %q", s.Pointer(), test.pointer)
			}

			b := scope.BaseURI()
			if b.String() != test.baseURI {
				t.Errorf("BaseURI: have %q, need %q", b, test.baseURI)
			}

			u := scope.URI()
			if u.String() != test.uri {
				t.Errorf("URI: have %q, need %q", u, test.uri)
			}
		})
	}
}
