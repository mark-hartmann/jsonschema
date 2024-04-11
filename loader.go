package jsonschema

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// UnsupportedURI is returned by a Loader to signal that the loader is
// unable to process the URI.
var UnsupportedURI = errors.New("unsupported URI")

type Loader interface {
	Load(ctx context.Context, uri *url.URL) (*Schema, error)
}

type LoaderFunc func(ctx context.Context, uri *url.URL) (*Schema, error)

func (f LoaderFunc) Load(ctx context.Context, uri *url.URL) (*Schema, error) {
	return f(ctx, uri)
}

// NewEmbeddedLoader returns a Loader that searches fs for the URI. This loader will
// return UnsupportedURI if the Scheme is not "file".
//
// Does not support distinct schema resources within a single schema document.
func NewEmbeddedLoader(fs embed.FS) Loader {
	return LoaderFunc(func(_ context.Context, uri *url.URL) (*Schema, error) {
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
