package jsonschema

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

type Step struct {
	// Keyword is the origin keyword of the current Schema node.
	Keyword string
	// Key is the map key if the schema is part of an object keyword such
	// as [Schema.Properties] or [Schema.Defs].
	Key *string
	// Index is the array index if the schema is part of an array keyword
	// such as [Schema.AnyOf].
	Index *int
}

func (s Step) String() string {
	if s.Key != nil {
		return s.Keyword + "/" + *s.Key
	} else if s.Index != nil {
		return s.Keyword + "/" + strconv.Itoa(*s.Index)
	}
	return s.Keyword
}

// MetaFunc creates a new meta instance for the provided scope.
type MetaFunc[T any] func(s *Scope[T]) *T

// NewScope creates a new root scope for the provided schema. The
// metaFactory is called each step and the returned value is set to the
// current scope.
func NewScope[T any](schema *Schema, metaFactory MetaFunc[T]) (*Scope[T], error) {
	s := &Scope[T]{Schema: schema, metaFunc: metaFactory}
	if schema != nil && schema.ID != "" {
		var err error
		s.baseUri, err = url.Parse(schema.ID)
		if err != nil {
			return s, fmt.Errorf("failed to parse resource schema id %s: %w", schema.ID, err)
		}
	}
	if metaFactory != nil {
		s.Meta = metaFactory(s)
	}
	return s, nil
}

type Scope[T any] struct {
	Parent *Scope[T]
	Schema *Schema
	Step

	// Meta contains contextual data.
	Meta *T
	// metaFunc creates a new meta instance for the provided
	// scope.
	metaFunc MetaFunc[T]

	// baseUri is only set at resource schemas.
	baseUri *url.URL
}

func (s *Scope[T]) Next(schema *Schema, step Step) (*Scope[T], error) {
	n := &Scope[T]{
		Parent:   s,
		Schema:   schema,
		Step:     step,
		metaFunc: s.metaFunc,
	}
	if schema != nil && schema.ID != "" {
		uri, err := url.Parse(schema.ID)
		if err != nil {
			return n, fmt.Errorf("failed to parse resource schema id %s: %w", schema.ID, err)
		}
		pr := n.ParentResource()
		if pr == nil {
			n.baseUri = uri
		} else {
			n.baseUri = pr.baseUri.ResolveReference(uri)
		}
	}

	if n.metaFunc != nil {
		n.Meta = n.metaFunc(n)
	}
	return n, nil
}

func (s *Scope[T]) atResource() bool {
	return s.Schema != nil && s.Schema.ID != ""
}

func (s *Scope[T]) buildPtr(fromRoot bool) string {
	var parts []string
	c := s
	for c != nil {
		if c.Keyword == "" || (!fromRoot && c.atResource()) {
			// If c sits at a resource schema, break the loop so the step TO
			// the resource is not added to the pointer
			break
		}
		parts = append(parts, c.Step.String())
		c = c.Parent
	}

	slices.Reverse(parts)
	return "/" + strings.Join(parts, "/")
}

// Pointer returns the pointer to the current scope relative to the previous
// resource schema. If there is no resource schema, the pointer is relative to
// the root node.
func (s *Scope[T]) Pointer() string {
	return s.buildPtr(false)
}

// PointerRoot returns the complete pointer relative to the root node, usually
// representing the JSON document root.
func (s *Scope[T]) PointerRoot() string {
	return s.buildPtr(true)
}

func (s *Scope[T]) URI() *url.URL {
	uri := url.URL{}
	if baseUri := s.BaseURI(); baseUri != nil {
		uri = *baseUri
	}
	if ptr := s.Pointer(); ptr != "/" {
		uri.Fragment = ptr
	}
	return &uri
}

// BaseURI returns the scopes current base uri.
func (s *Scope[T]) BaseURI() *url.URL {
	if s.baseUri != nil {
		return s.baseUri
	}
	res := s.ParentResource()
	if res == nil {
		return nil
	}
	return res.baseUri
}

func (s *Scope[T]) ParentResource() *Scope[T] {
	c := s.Parent
	for c != nil {
		if c.atResource() {
			return c
		}
		c = c.Parent
	}
	return nil
}
