package jsonschema

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func ResolveReference(loader LoaderFunc, ref string, schema *Schema) (*Schema, error) {
	u, err := url.Parse(ref)
	if err != nil {
		return nil, fmt.Errorf("foo: failed to parse $ref URI: %v", err)
	}

	var path []string

	switch {
	case u.IsAbs():
		if schema, err = loader(u); err != nil {
			return nil, fmt.Errorf("foo: failed to load external schema at %q: %w",
				u.String(), err)
		}
		fallthrough
	case u.Path == "":
		path = getUnescapedPath(u.Fragment)
	case u.Path != "":
		path = getUnescapedPath(u.Path)
	}

	return resolveRef(loader, path, 0, schema, schema, false)
}

// sideLoadSchema locates the reference pointer in schema. The references
// can be relative, relative to root or external.
func sideLoadSchema(loader LoaderFunc, root, schema *Schema) (*Schema, error) {
	u, err := url.Parse(schema.Ref)
	if err != nil {
		return nil, fmt.Errorf("failed to parse $ref URI: %v", err)
	}

	// No external lookup required because $ref refers either to the schema
	// itself or to its root. If $ref is relative, we set ignoreRef=true in
	// order to prevent infinite side-loading loop.
	if !u.IsAbs() && u.Path != "" {
		path := getUnescapedPath(u.Path)
		return resolveRef(loader, path, 0, root, schema, path == nil || isRelative(path))
	}

	next := root
	if u.IsAbs() {
		if next, err = loader(u); err != nil {
			return nil, fmt.Errorf("failed to load external schema at %q: %w",
				u.String(), err)
		}
	}

	return ResolveReference(loader, u.Fragment, next)
}

func resolveRef(loader LoaderFunc, path []string, pos int, root, schema *Schema,
	ignoreRef bool) (*Schema, error) {

	// Return if the current schema is not set, or we reached the end of
	// the reference path without the schema having a reference itself.
	if schema == nil || (len(path[pos:]) == 0 && schema.Ref == "") {
		return schema, nil
	}

	if schema.Ref != "" /* && schema.Ref != "#" */ && !ignoreRef {
		var err error
		if schema, err = sideLoadSchema(loader, root, schema); err != nil {
			return nil, fmt.Errorf("failed to side-load referenced schema: %w", err)
		}
	}

	if len(path[pos:]) == 0 {
		return schema, nil
	}

	segment := path[pos]
	switch segment {
	case "#":
		return resolveRef(loader, path, pos+1, root, root, false)
	case "allOf", "anyOf", "oneOf", "not", "prefixItems":
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
		case "not":
			col = schema.Not
		case "prefixItems":
			col = schema.PrefixItems
		}

		i, err := strconv.Atoi(nextSegment)
		if err != nil {
			return nil, fmt.Errorf("resolveRef: invalid array index %q: %w", nextSegment, err)
		} else if len(col) <= i {
			return nil, fmt.Errorf("resolveRef: index out of bounds %q", nextSegment)
		}

		return resolveRef(loader, path, pos+2, root, &col[i], false)
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
		return resolveRef(loader, path, pos+2, root, schema, false)
	case "if":
		return resolveRef(loader, path, pos+1, root, schema.If, false)
	case "then":
		return resolveRef(loader, path, pos+1, root, schema.Then, false)
	case "else":
		return resolveRef(loader, path, pos+1, root, schema.Else, false)
	case "items":
		return resolveRef(loader, path, pos+1, root, schema.Items, false)
	case "contains":
		return resolveRef(loader, path, pos+1, root, schema.Contains, false)
	case "additionalProperties":
		return resolveRef(loader, path, pos+1, root, schema.AdditionalProperties, false)
	case "propertyNames":
		return resolveRef(loader, path, pos+1, root, schema.PropertyNames, false)
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
