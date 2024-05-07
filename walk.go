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
type WalkFunc func(ptr string, schema *Schema) error

// Walk walks the schema tree rooted at root, calling fn for each schema, including
// root. The schemas are not walked in lexical order. The WalkFunc is first called
// with the current schema and then walked if no error occurred.
//
// If WalkFunc replaces the current schema, the new schema is walked:
//
//	func(ptr string, schema *Schema) error {
//	  // current schema is {"not":{}}
//	  *schema = Schema{AllOf: []Schema{/*...*/}}
//	  return nil
//	}
func Walk(root *Schema, fn WalkFunc) error {
	if err := fn("/", root); err != nil {
		if errors.Is(err, Skip) || errors.Is(err, SkipAll) {
			return nil
		} else {
			return err
		}
	}

	var walk func(string, *Schema, WalkFunc) error
	walk = func(prefix string, root *Schema, fn WalkFunc) error {
		var err error
		iter(root, func(ptr string, schema *Schema) (c bool) {
			p := path.Join(prefix, ptr)
			if err = fn("/"+p, schema); err != nil {
				// If fn returned Skip or SkipAll, reset the error and return early to
				// prevent walking the skipped schema. If the error is not the special
				// error Skip, c is set to false and the iteration stops.
				if skip := errors.Is(err, Skip); skip || errors.Is(err, SkipAll) {
					c, err = skip, nil
				}
				return c
			}

			err = walk(p, schema, fn)
			return err == nil
		})
		return err
	}
	return walk("", root, fn)
}

func iter(s *Schema, cont func(string, *Schema) bool) {
	for keyword, schema := range map[string]*Schema{
		"not":                  s.Not,
		"if":                   s.If,
		"then":                 s.Then,
		"else":                 s.Else,
		"items":                s.Items,
		"contains":             s.Contains,
		"additionalProperties": s.AdditionalProperties,
		"propertyNames":        s.PropertyNames,
	} {
		if schema == nil {
			continue
		}
		if !cont(keyword, schema) {
			return
		}
	}

	for keyword, schemas := range map[string][]Schema{
		"allOf":       s.AllOf,
		"anyOf":       s.AnyOf,
		"oneOf":       s.OneOf,
		"prefixItems": s.PrefixItems,
	} {
		for i := range schemas {
			if !cont(fmt.Sprintf("%s/%d", keyword, i), &schemas[i]) {
				return
			}
		}
	}

	for keyword, schemas := range map[string]map[string]Schema{
		"$defs":             s.Defs,
		"dependentSchemas":  s.DependentSchemas,
		"properties":        s.Properties,
		"patternProperties": s.PatternProperties,
	} {
		for name := range schemas {
			v := schemas[name]
			if !cont(fmt.Sprintf("%s/%s", keyword, name), &v) {
				schemas[name] = v
				return
			}
			schemas[name] = v
		}
	}
}
