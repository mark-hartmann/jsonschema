package jsonschema

import (
	"context"
	"net/url"
)

type Identifiers struct {
	BaseURI                 string
	CanonResourcePlainURI   string
	CanonResourcePointerURI string
	EnclosingResourceURIs   []string
}

// ComputeIdentifiers returns all schema identifiers defined in root's sub schemas, excluding
// root. The map key is a JSON pointer that points to the id defining schema.
func ComputeIdentifiers(root Schema) (map[string]Identifiers, error) {
	base, _ := url.Parse(root.ID)
	m := make(map[string]Identifiers)
	_ = Walk(context.Background(), &root, func(_ context.Context, scope Scope, s *Schema) error {
		// Copy the schema because we need to modify the ID for recursive calls.
		// Weak copy is enough.
		schema := *s

		if scope.Pointer == "/" || (schema.ID == "" && schema.Anchor == "") {
			return nil
		}

		var (
			err error
			ids Identifiers
		)

		if schema.ID != "" {
			id, _ := url.Parse(schema.ID)
			schema.ID = base.ResolveReference(id).String()

			m2, _ := ComputeIdentifiers(schema)
			for k, v := range m2 {
				encURI := base.String() + "#" + scope.Pointer + k
				v.EnclosingResourceURIs = append(v.EnclosingResourceURIs, encURI)

				m[scope.Pointer+k] = v
			}

			ids.BaseURI = base.ResolveReference(id).String()
			ids.CanonResourcePointerURI = ids.BaseURI + "#"
			err = Skip
		} else {
			ids.BaseURI = base.String()
			ids.CanonResourcePointerURI = ids.BaseURI + "#" + scope.Pointer
		}

		if schema.Anchor != "" {
			ids.CanonResourcePlainURI = ids.BaseURI + "#" + schema.Anchor
		}

		if encURI := base.String() + "#" + scope.Pointer; encURI != ids.CanonResourcePointerURI {
			ids.EnclosingResourceURIs = append(ids.EnclosingResourceURIs, encURI)
		}

		m[scope.Pointer] = ids
		return err
	})

	return m, nil
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
