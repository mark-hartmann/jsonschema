package jsonschema

import (
	"context"
	"net/url"
	"path"
	"slices"
)

type Identifiers struct {
	BaseURI                 string
	CanonResourcePlainURI   string
	CanonResourcePointerURI string
	EnclosingResourceURIs   []string
}

// computeScopeIds computes the Identifiers for the provided scope, including
// enclosing resource URIs. The returned value is nil if the scope does not point to
// a resource schema.
func computeScopeIds(s *Scope[Identifiers]) *Identifiers {
	// If scope is not at a resource and the schema has no anchor, return.
	if !s.atResource() && (s.Schema == nil || s.Schema.Anchor == "") {
		return nil
	}

	ids := Identifiers{}
	baseURI, _ := s.BaseURI()
	ids.BaseURI = baseURI.String()
	if s.atResource() {
		ids.CanonResourcePointerURI = ids.BaseURI + "#"
	} else {
		ids.CanonResourcePointerURI = ids.BaseURI + "#" + s.Pointer()
	}
	if s.Schema.Anchor != "" {
		ids.CanonResourcePlainURI = ids.BaseURI + "#" + s.Schema.Anchor
	}

	// s.Step is how we got to this schema, so we have to start from s as current
	steps := []string{s.Step.String()}
	cURI, _ := s.URI()
	c := s.Parent
	for c != nil {
		atResource := c.Schema != nil && c.Schema.ID != ""
		if atResource {
			uri, _ := c.BaseURI()
			prts := make([]string, len(steps))
			copy(prts, steps)
			slices.Reverse(prts)
			uri = uri.ResolveReference(&url.URL{Fragment: "/" + path.Join(prts...)})

			if cURI.String() != uri.String() {
				ids.EnclosingResourceURIs = append(ids.EnclosingResourceURIs, uri.String())
			}
		}
		steps = append(steps, c.Step.String())
		c = c.Parent
	}

	return &ids
}

// ComputeIdentifiers returns all schema identifiers defined in root's sub schemas, including
// root. The map key is a JSON pointer that points to the id defining schema.
func ComputeIdentifiers(root Schema) (map[string]Identifiers, error) {
	ids := make(map[string]Identifiers)
	ids["/"] = Identifiers{
		BaseURI:                 root.ID,
		CanonResourcePointerURI: root.ID + "#",
	}

	_ = Walk(context.Background(), &root, computeScopeIds, func(_ context.Context, scope *Scope[Identifiers], s *Schema) error {
		if scope.Meta != nil {
			ids[scope.PointerRoot()] = *scope.Meta
		}
		return nil
	})

	return ids, nil
}

// isEmbedded returns whether a URI is embedded, i.e. if the root schema resource
// embeds a schema resource with the same base URI. It does not check if the provided
// reference URI actually exists.
func isEmbedded(rawURI string, identifiers map[string]Identifiers) bool {
	uri, _ := url.Parse(rawURI)
	uri.Fragment = ""
	for _, id := range identifiers {
		if id.BaseURI == uri.String() {
			return true
		}
	}
	return false
}
