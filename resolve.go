package jsonschema

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type ResolveConfig struct {
	Context context.Context
	Loader  Loader

	resource            *Schema
	rootResource        *Schema
	rootResourceLoader  Loader
	resourceURI         *url.URL
	computedIdentifiers map[string]Identifiers
	ignoreRefs          bool
}

func applyDefaults(config *ResolveConfig, resource *Schema) {
	if config.Context == nil {
		config.Context = context.Background()
	}

	if config.Loader == nil {
		config.Loader = LoaderFunc(func(_ context.Context, uri *url.URL) (*Schema, error) {
			return nil, fmt.Errorf("no loader configured")
		})
	}

	if config.resource == nil {
		config.resource = resource
	}

	if config.resourceURI == nil {
		config.resourceURI, _ = url.Parse(resource.ID)
	}

	if config.rootResource == nil {
		config.rootResource = resource
		config.rootResourceLoader = NewLocalLoader(resource, nil)
		config.computedIdentifiers, _ = ComputeIdentifiers(*resource)
	}
}

// ResolveReference resolves a JSON reference pointer against the provided Schema.
// If the reference (or some node of it) points to an external URI, the loaders is
// used.
func ResolveReference(config ResolveConfig, ref string, resource *Schema) (*Schema, error) {
	applyDefaults(&config, resource)

	if resource.ID != "" {
		config.resource = resource

		uri, _ := url.Parse(resource.ID)
		config.resourceURI = config.resourceURI.ResolveReference(uri)

		// If ids are not computed or the resource ID is not embedded in the root
		// schema resource!
		if config.computedIdentifiers == nil || !isEmbedded(resource.ID, config.computedIdentifiers) {
			config.computedIdentifiers, _ = ComputeIdentifiers(*resource)
		}
	}

	uri, _ := url.Parse(ref)
	isPointerReference := len(ref) == 0 || len(ref) > 2 && ref[0] == '#' && ref[1] == '/'

	var path []string
	var refPath string
	if isPointerReference {
		refPath = uri.Fragment
	} else {
		uri = config.resourceURI.ResolveReference(uri)
		if isEmbedded(uri.String(), config.computedIdentifiers) {
			var ids Identifiers

			bURI, _ := url.Parse(uri.String())
			bURI.Fragment = ""
			for _, id := range config.computedIdentifiers {
				if id.BaseURI == uri.String() {
					ids = id
					break
				}
			}

			s, err := config.rootResourceLoader.Load(config.Context, uri)
			if err != nil {
				return nil, fmt.Errorf("unable to locate embedded resource: %w", err)
			}

			resource = s
			config.resource = s
			config.resourceURI, _ = url.Parse(ids.BaseURI)
		} else {
			s, err := config.Loader.Load(config.Context, uri)
			if err != nil {
				return nil, fmt.Errorf("unable to locate non-embedded resource {\"$id\": %q}: %w", uri, err)
			}
			return ResolveReference(ResolveConfig{Context: config.Context, Loader: config.Loader}, uri.String(), s)
		}

		if uri.Path != "" {
			refPath = uri.Path
		} else {
			refPath = uri.Fragment
		}
	}

	path = getUnescapedPath(refPath)
	if err := ValidateReferencePointer(refPath); err != nil {
		return nil, fmt.Errorf("invalid reference %s: %w", fmtPos(config, path, len(path)), err)
	}

	config.ignoreRefs = true
	return resolveRef(config, config.resource, path, 0)
}

func fmtPos(config ResolveConfig, path []string, pos int) string {
	var res string
	if uriStr := config.resourceURI.String(); uriStr != "" {
		res = uriStr
	} else {
		res = "<root>"
	}

	return fmt.Sprintf("%s%s", res, fmtPtrPosition(path, pos))
}

// fmtPtrPosition prints the resource schema id including the reference pointer
// up to pos. Use len(path) to append the whole pointer.
func fmtPtrPosition(path []string, pos int) string {
	var sb strings.Builder
	sb.WriteString("#/")
	for i := 0; i < pos; i++ {
		sb.WriteString(path[i])
		if i < pos-1 {
			sb.WriteString("/")
		}
	}
	return sb.String()
}

func resolveRef(config ResolveConfig, current *Schema, path []string, pos int) (*Schema, error) {
	// Return if the current schema is not set, or we reached the end of
	// the reference path without the schema having a reference itself.
	if current == nil || (len(path[pos:]) == 0 && current.Ref == "") {
		return current, nil
	}

	if current.ID != "" {
		uri, _ := url.Parse(current.ID)
		config.resource = current
		config.resourceURI = config.resourceURI.ResolveReference(uri)
	}

	if current.Ref != "" /* && schema.Ref != "#" */ && (!config.ignoreRefs && len(path[pos:]) == 0) {
		var err error
		r := current.Ref
		if current, err = ResolveReference(config, current.Ref, current); err != nil {
			return nil, fmt.Errorf("failed to resolve {\"$ref\": %q} at %q: %w", r, fmtPos(config, path, pos), err)
		}
	}

	if len(path[pos:]) == 0 {
		return current, nil
	}

	config.ignoreRefs = false

	var res *Schema
	s, pos, err := fastResolve(current, &res, path, pos)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %s: %w", fmtPos(config, path, len(path)), err)
	}
	if res != nil {
		config.resource = res
		uri, _ := url.Parse(current.ID)
		config.resourceURI = config.resourceURI.ResolveReference(uri)
	}

	if s.Ref != "" {
		return resolveRef(config, s, path, pos)
	}
	return s, err
}

// fastResolve follows the path until pos=len(path) or the path cannot be followed
// further. If a resource schema is detected during traversal of the path, it is
// written to res.
func fastResolve(schema *Schema, res **Schema, path []string, pos int) (*Schema, int, error) {
	// We reached the end of the path, return status quo.
	if len(path[pos:]) == 0 {
		return schema, pos, nil
	}

	// Only set resource schema if not root
	if pos != 0 && schema.ID != "" {
		*res = schema
	}

	segment := path[pos]
	switch segment {
	case "allOf", "anyOf", "oneOf", "prefixItems":
		var col []Schema
		switch segment {
		case "allOf":
			col = schema.AllOf
		case "anyOf":
			col = schema.AnyOf
		case "oneOf":
			col = schema.OneOf
		default:
			col = schema.PrefixItems
		}

		i, _ := strconv.Atoi(path[pos+1])
		if len(col) <= i {
			return nil, pos, fmt.Errorf("index out of bounds (%d/%d) at %q", i, len(col)-1, fmtPtrPosition(path, pos+1))
		}
		return fastResolve(&col[i], res, path, pos+2)
	case "$defs", "dependentSchemas", "properties", "patternProperties":
		var col map[string]Schema
		switch segment {
		case "$defs":
			col = schema.Defs
		case "dependentSchemas":
			col = schema.DependentSchemas
		case "properties":
			col = schema.Properties
		default:
			col = schema.PatternProperties
		}

		var (
			s  Schema
			ok bool
		)
		if s, ok = col[path[pos+1]]; !ok {
			return nil, pos, fmt.Errorf("unknown key %q at %q", path[pos+1], fmtPtrPosition(path, pos+1))
		}
		return fastResolve(&s, res, path, pos+2)
	case "not", "if", "then", "else", "items", "contains", "additionalProperties", "propertyNames", "unevaluatedItems", "unevaluatedProperties", "contentSchema":
		var s *Schema
		switch segment {
		case "not":
			s = schema.Not
		case "if":
			s = schema.If
		case "then":
			s = schema.Then
		case "else":
			s = schema.Else
		case "items":
			s = schema.Items
		case "contains":
			s = schema.Contains
		case "additionalProperties":
			s = schema.AdditionalProperties
		case "propertyNames":
			s = schema.PropertyNames
		case "unevaluatedItems":
			s = schema.UnevaluatedItems
		case "unevaluatedProperties":
			s = schema.UnevaluatedProperties
		default:
			s = schema.ContentSchema
		}

		if s == nil {
			return nil, pos, fmt.Errorf("expected non-nil schema at %q", fmtPtrPosition(path, pos+1))
		}
		return fastResolve(s, res, path, pos+1)
	}

	return nil, pos, fmt.Errorf("unknown segment %q", segment)
}

func getUnescapedPath(ref string) []string {
	ref = strings.TrimPrefix(ref, "/")

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
