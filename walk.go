package jsonschema

import (
	"errors"
	"fmt"
	"path"
)

var (
	Skip    = errors.New("skip this node")
	SkipAll = errors.New("skip everything and stop the walk")
)

// WalkFunc is called by Walk for each schema. The ptr argument contains the
// JSON pointer that points to the schema, starting from the root schema.
//
// The error result returned by the function controls how Walk continues.
// If the function returns the special error Skip, Walk skips any schemas
// defined in current node/schema, while SkipAll will skip all remaining schemas.
// If the function returns a non-nil error, Walk stops entirely and returns
// that error.
type WalkFunc func(ptr string, schema Schema) error

// Walk walks the schema tree rooted at root, calling fn for each schema, including
// root. The schemas are not walked in lexical order.
func Walk(root Schema, fn WalkFunc) error {
	if err := fn("/", root); err != nil {
		if errors.Is(err, Skip) || errors.Is(err, SkipAll) {
			return nil
		} else {
			return err
		}
	}
	return walk("", root, fn)
}

func walk(prefix string, root Schema, fn WalkFunc) error {
	m := make(map[string]Schema)

	for keyword, schemas := range map[string][]Schema{
		"allOf":       root.AllOf,
		"anyOf":       root.AnyOf,
		"oneOf":       root.OneOf,
		"prefixItems": root.PrefixItems,
	} {
		for i, s := range schemas {
			m[fmt.Sprintf("%s/%d", keyword, i)] = s
		}
	}

	for keyword, schema := range map[string]*Schema{
		"not":                  root.Not,
		"if":                   root.If,
		"then":                 root.Then,
		"else":                 root.Else,
		"items":                root.Items,
		"contains":             root.Contains,
		"additionalProperties": root.AdditionalProperties,
		"propertyNames":        root.PropertyNames,
	} {
		if schema != nil {
			m[keyword] = *schema
		}
	}

	for keyword, schemas := range map[string]map[string]Schema{
		"$defs":             root.Defs,
		"dependentSchemas":  root.DependentSchemas,
		"properties":        root.Properties,
		"patternProperties": root.PatternProperties,
	} {
		for name, s := range schemas {
			m[fmt.Sprintf("%s/%s", keyword, name)] = s
		}
	}

	for k, v := range m {
		k = path.Join(prefix, k)
		if err := fn("/"+k, v); err != nil {
			if errors.Is(err, SkipAll) {
				break
			} else if errors.Is(err, Skip) {
				continue
			}
			return err
		}

		if err := walk(k, v, fn); err != nil {
			return err
		}
	}
	return nil
}
