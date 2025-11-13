package jsonschema

import (
	"context"
	"errors"
	"path"
	"strconv"
)

var (
	Skip    = errors.New("skip this node")
	SkipAll = errors.New("skip everything and stop the walk")
)

type Scope struct {
	// Pointer is the JSON pointer that points to the schema, starting from
	// the current root schema.
	Pointer string
}

// WalkFunc is called by Walk for each schema.
//
// The error result returned by the function controls how Walk continues.
// If the function returns the special error Skip, Walk skips any schemas
// defined in current node/schema, while SkipAll will skip all remaining schemas.
// If the function returns a non-nil error, Walk stops entirely and returns
// that error.
type WalkFunc func(ctx context.Context, state Scope, schema *Schema) error

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
func Walk(ctx context.Context, schema *Schema, fn WalkFunc) error {
	scope := Scope{
		Pointer: "/",
	}
	if err := fn(ctx, scope, schema); err != nil {
		if errors.Is(err, Skip) || errors.Is(err, SkipAll) {
			return nil
		} else {
			return err
		}
	}
	return walkRec(ctx, scope, schema, fn)
}

func walkRec(ctx context.Context, scope Scope, schema *Schema, fn WalkFunc) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	var err error
	for _, n := range nodes(schema) {
		cScope := scope
		cScope.Pointer = path.Join(scope.Pointer, n.keyword)

		// If fn returns an error, it can be Skip or SkipAll or an actual error.
		if err = fn(ctx, cScope, n.schema); err != nil {
			var cont bool
			// If fn returned Skip or SkipAll, reset the error and return early to
			// prevent walking the skipped schema. If the error is not the special
			// error Skip, c is set to false and the iteration stops.
			if skip := errors.Is(err, Skip); skip || errors.Is(err, SkipAll) {
				// Skip and SkipAll are not really errors, so we null it
				cont, err = skip, nil
			}

			// cont is true only if the returned error is Skip, otherwise the error
			// is SkipAll or an actual error and will stop the walking.
			if !cont {
				return err
			}
			continue
		}

		n.set(*n.schema)
		err = walkRec(ctx, cScope, n.schema, fn)
		if err != nil {
			break
		}
	}
	return err
}

type node struct {
	keyword string
	schema  *Schema
	set     func(Schema)
}

var children = []struct {
	keyword string
	get     func(*Schema) *Schema
}{
	{"not", func(s *Schema) *Schema { return s.Not }},
	{"if", func(s *Schema) *Schema { return s.If }},
	{"then", func(s *Schema) *Schema { return s.Then }},
	{"else", func(s *Schema) *Schema { return s.Else }},
	{"items", func(s *Schema) *Schema { return s.Items }},
	{"contains", func(s *Schema) *Schema { return s.Contains }},
	{"additionalProperties", func(s *Schema) *Schema { return s.AdditionalProperties }},
	{"propertyNames", func(s *Schema) *Schema { return s.PropertyNames }},
	{"unevaluatedItems", func(s *Schema) *Schema { return s.UnevaluatedItems }},
	{"unevaluatedProperties", func(s *Schema) *Schema { return s.UnevaluatedProperties }},
	{"contentSchema", func(s *Schema) *Schema { return s.ContentSchema }},
}

func sliceChildren(keyword string, arr []Schema) []node {
	out := make([]node, len(arr))
	for i := range arr {
		i := i
		out[i] = node{
			keyword: keyword + "/" + strconv.Itoa(i),
			schema:  &arr[i],
			set: func(v Schema) {
				arr[i] = v
			},
		}
	}
	return out
}

func mapChildren(keyword string, m map[string]Schema) []node {
	out := make([]node, 0, len(m))
	for name, v := range m {
		name, v := name, v // capture
		out = append(out, node{
			keyword: keyword + "/" + name,
			schema:  &v,
			set: func(val Schema) {
				m[name] = val
			},
		})
	}
	return out
}

func nodes(s *Schema) []node {
	out := make([]node, 0, 16)
	for _, c := range children {
		if s1 := c.get(s); s1 == nil {
			continue
		}
		c := c
		out = append(out, node{
			keyword: c.keyword,
			schema:  c.get(s),
			set: func(s1 Schema) {
				*c.get(s) = s1
			},
		})
	}

	out = append(out, sliceChildren("allOf", s.AllOf)...)
	out = append(out, sliceChildren("anyOf", s.AnyOf)...)
	out = append(out, sliceChildren("oneOf", s.OneOf)...)
	out = append(out, sliceChildren("prefixItems", s.PrefixItems)...)

	out = append(out, mapChildren("$defs", s.Defs)...)
	out = append(out, mapChildren("dependentSchemas", s.DependentSchemas)...)
	out = append(out, mapChildren("properties", s.Properties)...)
	out = append(out, mapChildren("patternProperties", s.PatternProperties)...)

	return out
}
