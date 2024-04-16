package jsonschema

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type Identifiers struct {
	BaseURI                 string
	CanonResourcePlainURI   string
	CanonResourcePointerURI string
	EnclosingResourceURIs   []string
}

// ComputeIdentifiers returns all schema identifiers defined in root's subschemas, excluding
// root. The map key is a JSON pointer that points to the id defining schema.
func ComputeIdentifiers(root Schema) (map[string]Identifiers, error) {
	base, _ := url.Parse(root.ID)
	m := make(map[string]Identifiers)
	_ = Walk(root, func(ptr string, schema Schema) error {
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

// NewLocalLoader returns a loader that checks the URI against identifiable sub-schemas that
// are located within the root schema. If a sub-schema is found, the URI is replaced with
// a new URI relative to the resolved schema. If no schema is found, the next Loader is called.
//
// The identifiers are computed and prefetched only once.
func NewLocalLoader(root *Schema, next Loader) Loader {
	ids, _ := ComputeIdentifiers(*root)
	ids["/"] = Identifiers{
		BaseURI:                 root.ID,
		CanonResourcePointerURI: root.ID + "#",
	}

	prefetched := make(map[string]*Schema)
	for s, identifiers := range ids {
		if identifiers.BaseURI+"#" == identifiers.CanonResourcePointerURI {
			prefetched[identifiers.BaseURI], _ = resolveRef(context.Background(), nil,
				getUnescapedPath(s), 0, root, root, root, true)
		}
	}
	return LoaderFunc(func(ctx context.Context, uri *url.URL) (*Schema, error) {
		var (
			b, r string
			u    *url.URL
		)

		// search for direct match
		for _, id := range ids {
			if id.CanonResourcePlainURI == uri.String() {
				b = id.BaseURI
				// We parse res ptr uri because ids ptr is relative to document
				// root, we need relative to new current
				p, _ := url.Parse(id.CanonResourcePointerURI)
				r = p.Fragment
				break
			}
		}

		if b == "" {
			u2 := *uri
			u2.Fragment = ""
			for _, id := range ids {
				if id.BaseURI+"#" == id.CanonResourcePointerURI && id.BaseURI == u2.String() {
					b = id.BaseURI
					r = "#" + uri.Fragment
					break
				}
			}
		}

		if s, ok := prefetched[b]; ok && r != "" {
			if u, _ = url.Parse(r); u != nil {
				*uri = *u
			}
			return s, nil
		}
		return next.Load(ctx, uri)
	})
}

// ResolveReference resolves a JSON reference pointer against the provided Schema.
// If the reference (or some node of it) points to an external URI, the loaders is
// used.
func ResolveReference(ctx context.Context, loader Loader, ref string, current, root,
	documentRoot *Schema) (*Schema, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	u, err := url.Parse(ref)
	if err != nil {
		return nil, fmt.Errorf("foo: failed to parse $ref URI: %v", err)
	}

	docURI, _ := url.Parse(documentRoot.ID)
	rootURI, _ := url.Parse(root.ID)

	u = docURI.ResolveReference(rootURI).ResolveReference(u)

	// todo: It's probably best to cache this loader in the ctx and replace it if
	//       the documentRoot changes.
	localLoader := NewLocalLoader(documentRoot, loader)
	if ls, _ := localLoader.Load(ctx, u); ls != nil {
		current = ls
	}

	if current.ID != "" {
		root = current
	}

	var path []string

	ignoreRef := false
	switch {
	case u.IsAbs():
		if current, err = loader.Load(ctx, u); err != nil {
			return nil, fmt.Errorf("foo: failed to load external schema at %q: %w",
				u.String(), err)
		}
		root = current
		fallthrough
	case u.Path == "":
		// The URI contains no path, so we can assume it is relative to root, so [root]
		// is now our [schema]. For example:
		//
		//   #/$defs/foo
		//   file:///example/test.schema.json#properties/foo
		path = getUnescapedPath(u.Fragment)
		current = root
	case u.Path != "":
		// The URI is not absolute and the JSON pointer is not relative to
		// root, i.e. it's a relative pointer like "/properties/foo". This means
		// neither [schema] nor [root] changes.
		//
		// Schema are treated as READONLY, i.e. we can't remove the $ref from schema, which
		// is - if set - followed in [resolveRef], resulting in a stack overflow.
		path = getUnescapedPath(u.Path)
		ignoreRef = path == nil || isRelative(path)
	}

	return resolveRef(ctx, loader, path, 0, current, root, documentRoot, ignoreRef)
}

func resolveRef(ctx context.Context, loader Loader, path []string, pos int, current, root,
	docRoot *Schema, ignoreRef bool) (*Schema, error) {

	// Return if the current schema is not set, or we reached the end of
	// the reference path without the schema having a reference itself.
	if current == nil || (len(path[pos:]) == 0 && current.Ref == "") {
		return current, nil
	}

	if current.ID != "" && current != root {
		root = current
	}

	if current.Ref != "" /* && schema.Ref != "#" */ && !ignoreRef {
		var err error
		if current, err = ResolveReference(ctx, loader, current.Ref, current, root, docRoot); err != nil {
			return nil, fmt.Errorf("failed to side-load referenced schema: %w", err)
		}
	}

	if len(path[pos:]) == 0 {
		return current, nil
	}

	segment := path[pos]
	switch segment {
	case "#":
		return resolveRef(ctx, loader, path, pos+1, root, root, docRoot, false)
	case "allOf", "anyOf", "oneOf", "prefixItems":
		if len(path[pos:]) == 1 {
			return nil, fmt.Errorf("resolveRef: missing array index")
		}

		nextSegment := path[pos+1]

		var col []Schema
		switch segment {
		case "allOf":
			col = current.AllOf
		case "anyOf":
			col = current.AnyOf
		case "oneOf":
			col = current.OneOf
		case "prefixItems":
			col = current.PrefixItems
		}

		i, err := strconv.Atoi(nextSegment)
		if err != nil {
			return nil, fmt.Errorf("resolveRef: invalid array index %q: %w", nextSegment, err)
		} else if len(col) <= i {
			return nil, fmt.Errorf("resolveRef: index out of bounds %q", nextSegment)
		}

		return resolveRef(ctx, loader, path, pos+2, &col[i], root, docRoot, false)
	case "$defs", "dependentSchemas", "properties", "patternProperties":
		if len(path[pos:]) == 1 {
			return nil, fmt.Errorf("resolveRef: missing object key at %q", path[:pos+1])
		}

		var col map[string]Schema
		switch segment {
		case "$defs":
			col = current.Defs
		case "dependentSchemas":
			col = current.DependentSchemas
		case "properties":
			col = current.Properties
		case "patternProperties":
			col = current.PatternProperties
		}

		var (
			s  Schema
			ok bool
		)
		if s, ok = col[path[pos+1]]; !ok {
			return nil, fmt.Errorf("resolveRef: invalid path, no schema found at %q", path[:pos+2])
		}

		current = &s
		return resolveRef(ctx, loader, path, pos+2, current, root, docRoot, false)
	case "not":
		return resolveRef(ctx, loader, path, pos+1, current.Not, root, docRoot, false)
	case "if":
		return resolveRef(ctx, loader, path, pos+1, current.If, root, docRoot, false)
	case "then":
		return resolveRef(ctx, loader, path, pos+1, current.Then, root, docRoot, false)
	case "else":
		return resolveRef(ctx, loader, path, pos+1, current.Else, root, docRoot, false)
	case "items":
		return resolveRef(ctx, loader, path, pos+1, current.Items, root, docRoot, false)
	case "contains":
		return resolveRef(ctx, loader, path, pos+1, current.Contains, root, docRoot, false)
	case "additionalProperties":
		return resolveRef(ctx, loader, path, pos+1, current.AdditionalProperties, root, docRoot, false)
	case "propertyNames":
		return resolveRef(ctx, loader, path, pos+1, current.PropertyNames, root, docRoot, false)
	}

	return nil, fmt.Errorf("invalid path: unknown segment %q at %q", segment, path[:pos])
}

func isRelative(path []string) bool {
	return len(path) > 0 && path[0] != "#"
}

func getUnescapedPath(ref string) []string {
	ref = strings.TrimPrefix(ref, "/")
	ref = strings.TrimSuffix(ref, "/")

	if ref == "" {
		return nil
	}

	path := strings.Split(ref, "/")
	for i := range path {
		path[i] = strings.ReplaceAll(path[i], "~0", "~")
		path[i] = strings.ReplaceAll(path[i], "~1", "/")
	}

	return path
}
