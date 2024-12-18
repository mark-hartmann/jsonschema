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
	if isPointerReference {
		path = getUnescapedPath(uri.Fragment)
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
			path = getUnescapedPath(uri.Path)
		} else {
			path = getUnescapedPath(uri.Fragment)
		}
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
	segment := path[pos]
	switch segment {
	case "allOf", "anyOf", "oneOf", "prefixItems":
		if len(path[pos:]) == 1 {
			return nil, fmt.Errorf("missing array index at %q", fmtPos(config, path, pos+1))
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
			return nil, fmt.Errorf("invalid array index %q at %q: %w", nextSegment, fmtPos(config, path, pos+1), err)
		} else if len(col) <= i {
			return nil, fmt.Errorf("index out of bounds (%d/%d) at %q", i, len(col)-1, fmtPos(config, path, pos+1))
		}

		return resolveRef(config, &col[i], path, pos+2)
	case "$defs", "dependentSchemas", "properties", "patternProperties":
		if len(path[pos:]) == 1 {
			return nil, fmt.Errorf("missing key at %q", fmtPos(config, path, pos+1))
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
			return nil, fmt.Errorf("unknown key %q at %q", path[pos+1], fmtPos(config, path, pos+1))
		}

		current = &s
		return resolveRef(config, current, path, pos+2)
	case "not", "if", "then", "else", "items", "contains", "additionalProperties", "propertyNames":
		var s *Schema
		switch segment {
		case "not":
			s = current.Not
		case "if":
			s = current.If
		case "then":
			s = current.Then
		case "else":
			s = current.Else
		case "items":
			s = current.Items
		case "contains":
			s = current.Contains
		case "additionalProperties":
			s = current.AdditionalProperties
		case "propertyNames":
			s = current.PropertyNames
		case "unevaluatedItems":
			s = current.UnevaluatedItems
		case "unevaluatedProperties":
			s = current.UnevaluatedProperties
		case "contentSchema":
			s = current.ContentSchema
		}

		if s == nil {
			return nil, fmt.Errorf("missing schema at %q", fmtPos(config, path, pos+1))
		}
		return resolveRef(config, s, path, pos+1)
	}
	return nil, fmt.Errorf("unknown keyword %q at %q", segment, fmtPos(config, path, pos))
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
