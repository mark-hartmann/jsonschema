package jsonschema

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ResolveReference resolves a JSON reference pointer against the provided Schema.
// If the reference (or some node of it) points to an external URI, the loaders is
// used.
func ResolveReference(ctx context.Context, loader Loader, ref string, schema, root *Schema) (*Schema, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	u, err := url.Parse(ref)
	if err != nil {
		return nil, fmt.Errorf("foo: failed to parse $ref URI: %v", err)
	}

	var path []string

	ignoreRef := false
	switch {
	case u.IsAbs():
		if schema, err = loader.Load(ctx, u); err != nil {
			return nil, fmt.Errorf("foo: failed to load external schema at %q: %w",
				u.String(), err)
		}
		root = schema
		fallthrough
	case u.Path == "":
		// The URI contains no path, so we can assume it is relative to root, so [root]
		// is now our [schema]. For example:
		//
		//   #/$defs/foo
		//   file:///example/test.schema.json#properties/foo
		path = getUnescapedPath(u.Fragment)
		schema = root
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

	return resolveRef(ctx, loader, path, 0, root, schema, ignoreRef)
}

func resolveRef(ctx context.Context, loader Loader, path []string, pos int, root, schema *Schema,
	ignoreRef bool) (*Schema, error) {

	// Return if the current schema is not set, or we reached the end of
	// the reference path without the schema having a reference itself.
	if schema == nil || (len(path[pos:]) == 0 && schema.Ref == "") {
		return schema, nil
	}

	if schema.Ref != "" /* && schema.Ref != "#" */ && !ignoreRef {
		var err error
		if schema, err = ResolveReference(ctx, loader, schema.Ref, schema, root); err != nil {
			return nil, fmt.Errorf("failed to side-load referenced schema: %w", err)
		}
	}

	if len(path[pos:]) == 0 {
		return schema, nil
	}

	segment := path[pos]
	switch segment {
	case "#":
		return resolveRef(ctx, loader, path, pos+1, root, root, false)
	case "allOf", "anyOf", "oneOf", "prefixItems":
		if len(path[pos:]) == 1 {
			return nil, fmt.Errorf("resolveRef: missing array index")
		}

		nextSegment := path[pos+1]

		var col []Schema
		switch segment {
		case "allOf":
			col = schema.AllOf
		case "anyOf":
			col = schema.AnyOf
		case "oneOf":
			col = schema.OneOf
		case "prefixItems":
			col = schema.PrefixItems
		}

		i, err := strconv.Atoi(nextSegment)
		if err != nil {
			return nil, fmt.Errorf("resolveRef: invalid array index %q: %w", nextSegment, err)
		} else if len(col) <= i {
			return nil, fmt.Errorf("resolveRef: index out of bounds %q", nextSegment)
		}

		return resolveRef(ctx, loader, path, pos+2, root, &col[i], false)
	case "$defs", "dependentSchemas", "properties", "patternProperties":
		if len(path[pos:]) == 1 {
			return nil, fmt.Errorf("resolveRef: missing object key at %q", path[:pos+1])
		}

		var col map[string]Schema
		switch segment {
		case "$defs":
			col = schema.Defs
		case "dependentSchemas":
			col = schema.DependentSchemas
		case "properties":
			col = schema.Properties
		case "patternProperties":
			col = schema.PatternProperties
		}

		var (
			s  Schema
			ok bool
		)
		if s, ok = col[path[pos+1]]; !ok {
			return nil, fmt.Errorf("resolveRef: invalid path, no schema found at %q", path[:pos+2])
		}

		schema = &s
		return resolveRef(ctx, loader, path, pos+2, root, schema, false)
	case "not":
		return resolveRef(ctx, loader, path, pos+1, root, schema.Not, false)
	case "if":
		return resolveRef(ctx, loader, path, pos+1, root, schema.If, false)
	case "then":
		return resolveRef(ctx, loader, path, pos+1, root, schema.Then, false)
	case "else":
		return resolveRef(ctx, loader, path, pos+1, root, schema.Else, false)
	case "items":
		return resolveRef(ctx, loader, path, pos+1, root, schema.Items, false)
	case "contains":
		return resolveRef(ctx, loader, path, pos+1, root, schema.Contains, false)
	case "additionalProperties":
		return resolveRef(ctx, loader, path, pos+1, root, schema.AdditionalProperties, false)
	case "propertyNames":
		return resolveRef(ctx, loader, path, pos+1, root, schema.PropertyNames, false)
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
