package jsonptr

import (
	"errors"
	"fmt"
	"strings"
)

// SegmentError represents an error that occurred in a segment of a path.
type SegmentError struct {
	Seg string // The segment of the path that caused the error.
	Pos int    // The position of the segment.
	Err error
}

func (e *SegmentError) Error() string {
	return fmt.Sprintf("invalid segment %q: %s", e.Seg, e.Err)
}

func (e *SegmentError) Unwrap() error {
	return e.Err
}

func (e *SegmentError) Is(err error) bool {
	if e.Err == err {
		return true
	}
	return errors.Is(e.Unwrap(), e.Err)
}

// EscapeSequenceError is an error indicating that an invalid escape sequence was
// encountered. This error is returned if a segment contains a tilde that is not
// followed by either a 0 or a 1.
type EscapeSequenceError string

func (e EscapeSequenceError) Error() string {
	return "invalid escape sequence: " + string(e)
}

type InvalidJSONPointerError string

func (e InvalidJSONPointerError) Error() string {
	return "invalid JSON pointer: " + string(e)
}

type InvalidIndexError string

func (e InvalidIndexError) Error() string {
	return fmt.Sprintf("invalid array index: %q", string(e))
}

// ValidateJSONPointerFunc validates a string according to RFC 6901 and checks the
// individual pointer segments using fn, which is called after a segment has been
// validated. All segments are treated as object properties; array indices must be
// checked manually.
func ValidateJSONPointerFunc(pointer string, fn func(int, []string) error) error {
	if len(pointer) == 0 || (len(pointer) == 1 && pointer[0] == '/') {
		return nil
	}

	if pointer[0] != '/' {
		return InvalidJSONPointerError(pointer)
	}

	// The first char must be a "/", so we ignore the first occurrence.
	// Following "/" are kept, as "//" is a valid JSON pointer.
	path := strings.Split(pointer[1:], "/")

	for i, segment := range path {
		token := []rune(segment)
		for j := 0; j < len(token); j++ {
			// A reference token is *(unescaped / escaped) where unescaped is any
			// of (0x00-2E / 0x30-7D / 0x7F-10FFFF), practically every code point
			// except ~ and /, both of which are covered.
			if token[j] != '~' || (j < len(token)-1 && (token[j+1] == '0' || token[j+1] == '1')) {
				continue
			}

			s := token[j : j+1]
			if j != len(token)-1 {
				s = append(s, token[j+1])
			}

			return &SegmentError{Seg: segment, Pos: i, Err: EscapeSequenceError(s)}
		}

		if fn != nil {
			if err := fn(i, path); err != nil {
				return err
			}
		}
	}

	return nil
}

// IsArrayIndex checks if a segment is a valid JSON pointer array index.
func IsArrayIndex(segment string) bool {
	r := []rune(segment)
	if len(r) == 1 && r[0] == '0' {
		return true
	}

	for j := 0; j < len(r); j++ {
		if (j == 0 && r[j] == '0') || (r[j] < '0' || r[j] > '9') {
			return false
		}
	}
	return true
}
