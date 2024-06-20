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

// NewLocalLoader returns a loader that checks the URI against identifiable sub-schemas that
// are located within the root schema. If a sub-schema is found, the URI is replaced with
// a new URI relative to the resolved schema. If no schema is found, the next Loader is called.
//
// The identifiers are computed and prefetched only once.
func NewLocalLoader(root *Schema, next Loader) Loader {
	ids, _ := ComputeIdentifiers(*root)
	ids["/"] = Identifiers{
		BaseURI:                 root.ID,
		CanonResourcePointerURI: root.ID + "#",
	}

	prefetched := make(map[string]*Schema)
	for s, identifiers := range ids {
		if identifiers.BaseURI+"#" == identifiers.CanonResourcePointerURI {
			prefetched[identifiers.BaseURI], _ = resolveRef(ResolveConfig{ignoreRefs: true}, root,
				getUnescapedPath(s), 0)
		}
	}
	return LoaderFunc(func(ctx context.Context, uri *url.URL) (*Schema, error) {
		var (
			b, r string
			u    *url.URL
		)

		// search for direct match
		for _, id := range ids {
			if id.CanonResourcePlainURI == uri.String() {
				b = id.BaseURI
				// We parse res ptr uri because ids ptr is relative to document
				// root, we need relative to new current
				p, _ := url.Parse(id.CanonResourcePointerURI)
				r = p.Fragment
				break
			}
		}

		if b == "" {
			u2 := *uri
			u2.Fragment = ""
			for _, id := range ids {
				if id.BaseURI+"#" == id.CanonResourcePointerURI && id.BaseURI == u2.String() {
					b = id.BaseURI
					r = "#" + uri.Fragment
					break
				}
			}
		}

		if s, ok := prefetched[b]; ok && r != "" {
			if u, _ = url.Parse(r); u != nil {
				*uri = *u
			}
			return s, nil
		}

		if next != nil {
			return next.Load(ctx, uri)
		}
		return nil, nil
	})
}
