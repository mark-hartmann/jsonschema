package jsonschema

import "net/url"

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
	_ = Walk(&root, func(ptr string, s *Schema) error {
		// Copy the schema because we need to modify the ID for recursive calls.
		// Weak copy is enough.
		schema := *s

		if ptr == "/" || (schema.ID == "" && schema.Anchor == "") {
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
				encURI := base.String() + "#" + ptr + k
				v.EnclosingResourceURIs = append(v.EnclosingResourceURIs, encURI)

				m[ptr+k] = v
			}

			ids.BaseURI = base.ResolveReference(id).String()
			ids.CanonResourcePointerURI = ids.BaseURI + "#"
			err = Skip
		} else {
			ids.BaseURI = base.String()
			ids.CanonResourcePointerURI = ids.BaseURI + "#" + ptr
		}

		if schema.Anchor != "" {
			ids.CanonResourcePlainURI = ids.BaseURI + "#" + schema.Anchor
		}

		if encURI := base.String() + "#" + ptr; encURI != ids.CanonResourcePointerURI {
			ids.EnclosingResourceURIs = append(ids.EnclosingResourceURIs, encURI)
		}

		m[ptr] = ids
		return err
	})

	return m, nil
}
