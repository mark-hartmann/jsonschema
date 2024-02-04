package jsonschema

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// UnsupportedURI is returned by a LoaderFunc to signal that the loader function
// is unable so process the URI.
var UnsupportedURI = errors.New("unsupported URI")

type LoaderFunc func(uri *url.URL) (*Schema, error)

type Loader struct {
	loaders []LoaderFunc
	schemas map[string]*Schema
}

func (l Loader) Load(uri *url.URL) (*Schema, error) {

	f := uri.Fragment
	uri.Fragment = ""
	key := uri.String()
	uri.Fragment = f

	if schema, ok := l.schemas[key]; ok {
		return schema, nil
	}

	for _, fn := range l.loaders {
		schema, err := fn(uri)
		if err != nil {
			if errors.Is(err, UnsupportedURI) {
				continue
			}
			return nil, fmt.Errorf("jsonschema.Loader: failed to retrieve schema: %w", err)
		}

		l.schemas[key] = schema
		break
	}

	return l.schemas[key], nil
}

type LoaderOption func(*Loader)

func WithLoader(loader LoaderFunc) LoaderOption {
	return func(l *Loader) {
		l.loaders = append(l.loaders, loader)
	}
}

func NewLoader(opts ...LoaderOption) *Loader {
	l := &Loader{schemas: map[string]*Schema{}}
	for _, opt := range opts {
		opt(l)
	}

	return l
}

// NewEmbeddedLoader returns a LoaderFunc that searches fs for the URI.
func NewEmbeddedLoader(fs embed.FS) LoaderFunc {
	return func(uri *url.URL) (*Schema, error) {
		if uri.Scheme != "file" {
			return nil, UnsupportedURI
		}

		d, err := fs.ReadFile(strings.TrimPrefix(uri.Path, "/"))
		if err != nil {
			return nil, err
		}

		s := &Schema{}
		if err = json.Unmarshal(d, s); err != nil {
			return nil, fmt.Errorf("failed to read schema: %w", err)
		}

		return s, nil
	}
}
