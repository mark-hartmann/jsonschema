package jsonschema

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// UnsupportedURI is returned by a Loader to signal that the loaders is
// unable so process the URI.
var UnsupportedURI = errors.New("unsupported URI")

type Loader interface {
	Load(uri *url.URL) (*Schema, error)
}

type LoaderFunc func(uri *url.URL) (*Schema, error)

func (f LoaderFunc) Load(uri *url.URL) (*Schema, error) {
	return f(uri)
}

// NewEmbeddedLoader returns a Loader that searches fs for the URI.
func NewEmbeddedLoader(fs embed.FS) Loader {
	return LoaderFunc(func(uri *url.URL) (*Schema, error) {
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
	})
}
