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

		*uri = url.URL{Fragment: uri.Fragment}

		s := &Schema{}
		if err = json.Unmarshal(d, s); err != nil {
			return nil, fmt.Errorf("failed to read schema: %w", err)
		}

		return s, nil
	})
}

type LocalLoader struct {
	ids        map[string]Identifiers
	next       Loader
	prefetched map[string]*Schema
}

func (l *LocalLoader) Load(ctx context.Context, uri *url.URL) (*Schema, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var (
		baseURI, ref string
		u            *url.URL
	)
	// search for direct match
	for _, id := range l.ids {
		if id.CanonResourcePlainURI == uri.String() {
			baseURI = id.BaseURI
			// We parse res ptr uri because ids ptr is relative to document
			// root, we need relative to new current
			p, _ := url.Parse(id.CanonResourcePointerURI)
			ref = p.Fragment
			break
		}
	}

	if baseURI == "" && ref == "" {
		u2 := *uri
		u2.Fragment = ""
		for _, id := range l.ids {
			if id.BaseURI+"#" == id.CanonResourcePointerURI && id.BaseURI == u2.String() {
				baseURI = id.BaseURI
				ref = "#" + uri.Fragment
				break
			}
		}
	}

	if len(ref) > 1 && isNCName(ref[1:]) {
		return nil, fmt.Errorf("unknown anchor %q at %q", ref[1:], baseURI)
	}

	if s, ok := l.prefetched[baseURI]; ok && ref != "" {
		if u, _ = url.Parse(ref); u != nil {
			*uri = *u
		}
		return s, nil
	}

	if l.next != nil {
		s, err := l.next.Load(ctx, uri)
		if err != nil {
			return nil, fmt.Errorf("failed to load non-embedded resource {\"$id\": %q}: %w", uri, err)
		}
		return s, nil
	}
	return nil, nil
}

// NewLocalLoader returns a loader that checks the URI against identifiable sub-schemas that
// are located within the root schema. If a sub-schema is found, the URI is replaced with
// a new URI relative to the resolved schema. If no schema is found, the next Loader is called.
//
// The identifiers are computed and prefetched only once.
func NewLocalLoader(root *Schema, next Loader) *LocalLoader {
	ids, _ := ComputeIdentifiers(*root)
	l := &LocalLoader{ids: ids, next: next}

	l.prefetched = make(map[string]*Schema)
	for s, identifiers := range ids {
		if identifiers.BaseURI+"#" == identifiers.CanonResourcePointerURI {
			var rs *Schema
			l.prefetched[identifiers.BaseURI], _, _ = fastResolve(root, &rs, getUnescapedPath(s), 0)
		}
	}

	return l
}
