package jsonschema

import (
	"errors"
	"jsonschema/jsonptr"
)

// ErrPtrUnknownKeyword is a sentinel error indicating that an unknown keyword was
// encountered. This error is specific to the context of applicators as described
// in the [applicator] meta-schema/vocabulary, including the non-applicator
// keyword $defs.
//
// [applicator]: https://json-schema.org/draft/2020-12/json-schema-core#section-10
var ErrPtrUnknownKeyword = errors.New("unknown keyword")

// ErrPtrNoSchema is a sentinel error indicating that the final segment of a $ref
// pointer does not point to a schema.
var ErrPtrNoSchema = errors.New("does not point to schema")

// ValidateReferencePointer validates a schema reference pointer.
func ValidateReferencePointer(ref string) error {
	if len(ref) > 1 && ref[0] == '#' {
		if ref = ref[1:]; isNCName(ref) {
			return nil
		}
	}
	return jsonptr.ValidateJSONPointerFunc(ref, schemaSegmentValidator)
}

func isNCName(str string) bool {
	r := []rune(str)
	for i := 0; i < len(r); i++ {
		isNumber := r[i] >= '0' && r[i] <= '9'
		isLetter := (r[i] >= 'A' && r[i] <= 'Z') || (r[i] >= 'a' && r[i] <= 'z')

		if !isLetter && !isNumber && r[i] != '_' {
			if i > 0 && (r[i] == '-' || r[i] == '.') {
				continue
			}
			return false
		}
	}
	return true
}

func schemaSegmentValidator(i int, segments []string) error {
	segment := segments[i]
	switch segment {
	case "allOf", "anyOf", "oneOf", "prefixItems":
		fallthrough
	case "$defs", "dependentSchemas", "properties", "patternProperties":
		if i >= len(segments)-1 {
			return &jsonptr.SegmentError{Seg: segment, Pos: i, Err: ErrPtrNoSchema}
		}

		return nil
	case "not", "if", "then", "else", "items", "contains", "additionalProperties", "propertyNames",
		"unevaluatedItems", "unevaluatedProperties", "contentSchema":
		return nil
	default:
		if i <= 0 {
			break
		}

		prev := segments[i-1]
		switch prev {
		case "$defs", "dependentSchemas", "properties", "patternProperties":
			return nil
		case "allOf", "anyOf", "oneOf", "prefixItems":
			if !jsonptr.IsArrayIndex(segment) {
				return &jsonptr.SegmentError{
					Seg: segments[i], Pos: i, Err: jsonptr.InvalidIndexError(segment),
				}
			}
			return nil
		}
	}

	return &jsonptr.SegmentError{Seg: segments[i], Pos: i, Err: ErrPtrUnknownKeyword}
}
