package jsonschema

import (
	"bytes"
	"encoding/json"
)

type Type string

const (
	TypeNull    Type = "null"
	TypeBoolean Type = "boolean"
	TypeObject  Type = "object"
	TypeArray   Type = "array"
	TypeNumber  Type = "number"
	TypeInteger Type = "integer"
	TypeString  Type = "string"
)

type TypeSet []Type

func (s *TypeSet) UnmarshalJSON(b []byte) error {
	if b[0] == '[' {
		type ts TypeSet
		out := (*ts)(s)
		return json.Unmarshal(b, out)
	} else {
		var t Type
		err := json.Unmarshal(b, &t)
		if err != nil {
			*s = nil
		} else {
			*s = []Type{t}
		}
		return err
	}
}

var (
	True  = Schema{}
	False = Schema{Not: []Schema{True}}
)

type Schema struct {
	// Core
	Schema     string            `json:"$schema,omitempty"`
	Vocabulary map[string]bool   `json:"$vocabulary,omitempty"`
	ID         string            `json:"$id,omitempty"`
	Ref        string            `json:"$ref,omitempty"`
	DynamicRef string            `json:"$dynamicRef,omitempty"`
	Defs       map[string]Schema `json:"$defs,omitempty"`
	Comment    string            `json:"$comment,omitempty"`

	// Applying subschemas with logic
	AllOf []Schema `json:"allOf,omitempty"`
	AnyOf []Schema `json:"anyOf,omitempty"`
	OneOf []Schema `json:"oneOf,omitempty"`
	Not   []Schema `json:"not,omitempty"`

	// Applying subschemas conditionally
	If               *Schema           `json:"if,omitempty"`
	Then             *Schema           `json:"then,omitempty"`
	Else             *Schema           `json:"else,omitempty"`
	DependentSchemas map[string]Schema `json:"dependentSchemas,omitempty"`

	// Applying subschemas to arrays
	PrefixItems []Schema `json:"prefixItems,omitempty"`
	Items       *Schema  `json:"items,omitempty"`
	Contains    *Schema  `json:"contains,omitempty"`

	// Applying subschemas to objects
	Properties           map[string]Schema `json:"properties,omitempty"`
	PatternProperties    map[string]Schema `json:"patternProperties,omitempty"`
	AdditionalProperties *Schema           `json:"additionalProperties,omitempty"`
	PropertyNames        *Schema           `json:"propertyNames,omitempty"`

	// Validation
	Type  TypeSet `json:"type,omitempty"`
	Enum  []any   `json:"enum,omitempty"`
	Const any     `json:"const,omitempty"`

	// Validation for numbers
	MultipleOf       *json.Number `json:"multipleOf,omitempty"`
	Maximum          *json.Number `json:"maximum,omitempty"`
	ExclusiveMaximum *json.Number `json:"exclusiveMaximum,omitempty"`
	Minimum          *json.Number `json:"minimum,omitempty"`
	ExclusiveMinimum *json.Number `json:"exclusiveMinimum,omitempty"`

	// Validation for strings
	MaxLength *int    `json:"maxLength,omitempty"`
	MinLength *int    `json:"minLength,omitempty"`
	Pattern   *string `json:"pattern,omitempty"`

	// Validation for arrays
	MaxItems    *int  `json:"maxItems,omitempty"`
	MinItems    *int  `json:"minItems,omitempty"`
	UniqueItems *bool `json:"uniqueItems,omitempty"`
	MaxContains *int  `json:"maxContains,omitempty"`
	MinContains *int  `json:"minContains,omitempty"`

	// Validation for objects
	MaxProperties     *int                `json:"maxProperties,omitempty"`
	MinProperties     *int                `json:"minProperties,omitempty"`
	Required          []string            `json:"required,omitempty"`
	DependentRequired map[string][]string `json:"dependentRequired,omitempty"`

	// Basic metadata annotations
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Default     any    `json:"default,omitempty"`
	Deprecated  *bool  `json:"deprecated,omitempty"`
	ReadOnly    *bool  `json:"readOnly,omitempty"`
	WriteOnly   *bool  `json:"writeOnly,omitempty"`
	Examples    []any  `json:"examples,omitempty"`
}

func (s *Schema) String() string {
	res, _ := json.Marshal(s)
	return string(res)
}

func (s *Schema) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte("true")) {
		*s = Schema{}
	} else if bytes.Equal(b, []byte("false")) {
		*s = Schema{Not: []Schema{
			{},
		}}
	} else {
		type rawSchema Schema
		var out rawSchema
		if err := json.Unmarshal(b, &out); err != nil {
			return err
		}
		*s = Schema(out)
	}
	return nil
}

func (s *Schema) IsTrue() bool {
	return len(s.AllOf) == 0 &&
		len(s.AnyOf) == 0 &&
		len(s.OneOf) == 0 &&
		len(s.Not) == 0 &&
		s.If == nil &&
		s.Then == nil &&
		s.Else == nil &&
		len(s.DependentSchemas) == 0 &&
		len(s.PrefixItems) == 0 &&
		s.Items == nil &&
		s.Contains == nil &&
		len(s.Properties) == 0 &&
		len(s.PatternProperties) == 0 &&
		s.AdditionalProperties == nil &&
		s.PropertyNames == nil
}

func (s *Schema) IsFalse() bool {
	for _, not := range s.Not {
		if not.IsTrue() {
			return true
		}
	}
	return false
}
