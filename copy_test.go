package jsonschema_test

import (
	"encoding/json"
	. "jsonschema"
	"maps"
	"slices"
	"testing"
)

func TestCopy(t *testing.T) {
	s := Schema{
		Defs:                 map[string]Schema{"_": {Not: &Schema{}}},
		AllOf:                []Schema{{Not: &Schema{}}},
		AnyOf:                []Schema{{Not: &Schema{}}},
		OneOf:                []Schema{{Not: &Schema{}}},
		Not:                  &Schema{},
		If:                   &Schema{},
		Then:                 &Schema{},
		Else:                 &Schema{},
		DependentSchemas:     map[string]Schema{"_": {Not: &Schema{}}},
		PrefixItems:          []Schema{{Not: &Schema{}}},
		Items:                &Schema{},
		Contains:             &Schema{},
		Properties:           map[string]Schema{"_": {Not: &Schema{}}},
		PatternProperties:    map[string]Schema{"_": {Not: &Schema{}}},
		AdditionalProperties: &Schema{},
		PropertyNames:        &Schema{},
		Enum:                 []any{&Schema{}},
		Const:                &Schema{},
		MultipleOf:           ptr(json.Number("0")),
		Maximum:              ptr(json.Number("0")),
		ExclusiveMaximum:     ptr(json.Number("0")),
		Minimum:              ptr(json.Number("0")),
		ExclusiveMinimum:     ptr(json.Number("0")),
		MaxLength:            ptr(0),
		MinLength:            ptr(0),
		Pattern:              ptr(""),
		MaxItems:             ptr(0),
		MinItems:             ptr(0),
		UniqueItems:          ptr(false),
		MaxContains:          ptr(0),
		MinContains:          ptr(0),
		MaxProperties:        ptr(0),
		MinProperties:        ptr(0),
		Default:              &Schema{},
		DependentRequired:    map[string][]string{"_": {}},
		Deprecated:           ptr(false),
		ReadOnly:             ptr(false),
		WriteOnly:            ptr(false),
		Examples:             []any{&Schema{}},
	}
	c := Copy(s)

	for keyword, schema := range map[string][2]map[string]Schema{
		"$defs":             {s.Defs, c.Defs},
		"dependentSchemas":  {s.DependentSchemas, c.DependentSchemas},
		"Properties":        {s.Properties, c.Properties},
		"patternProperties": {s.PatternProperties, c.PatternProperties},
	} {
		if maps.EqualFunc(schema[0], schema[1], func(s1, s2 Schema) bool {
			return s1.Not == s2.Not
		}) {
			t.Logf("%s contains (sub)schema with the same memory address.", keyword)
			t.FailNow()
		}
	}

	for keyword, schema := range map[string][2][]Schema{
		"allOf":       {s.AllOf, c.AllOf},
		"anyOf":       {s.AnyOf, c.AnyOf},
		"oneOf":       {s.OneOf, c.OneOf},
		"prefixItems": {s.PrefixItems, c.PrefixItems},
	} {
		if slices.EqualFunc(schema[0], schema[1], func(s1, s2 Schema) bool {
			return s1.Not == s2.Not
		}) {
			t.Logf("%s contains (sub)schema with the same memory address.", keyword)
			t.FailNow()
		}
	}

	for keyword, val := range map[string][2]any{
		"not":                  {s.Not, c.Not},
		"if":                   {s.If, c.If},
		"then":                 {s.Then, c.Then},
		"else":                 {s.Else, c.Else},
		"items":                {s.Items, c.Items},
		"contains":             {s.Contains, c.Contains},
		"additionalProperties": {s.AdditionalProperties, c.AdditionalProperties},
		"propertyNames":        {s.PropertyNames, c.PropertyNames},
		"const":                {s.Const, c.Const},
		"multipleOf":           {s.MultipleOf, c.MultipleOf},
		"maximum":              {s.Maximum, c.Maximum},
		"exclusiveMaximum":     {s.ExclusiveMaximum, c.ExclusiveMaximum},
		"minimum":              {s.Minimum, c.Minimum},
		"exclusiveMinimum":     {s.ExclusiveMinimum, c.ExclusiveMinimum},
		"maxLength":            {s.MaxLength, c.MaxLength},
		"minLength":            {s.MinLength, c.MinLength},
		"pattern":              {s.Pattern, c.Pattern},
		"maxItems":             {s.MaxItems, c.MaxItems},
		"minItems":             {s.MinItems, c.MinItems},
		"uniqueItems":          {s.UniqueItems, c.UniqueItems},
		"maxContains":          {s.MaxContains, c.MaxContains},
		"minContains":          {s.MinContains, c.MinContains},
		"maxProperties":        {s.MaxProperties, c.MaxProperties},
		"minProperties":        {s.MinProperties, c.MinProperties},
		"default":              {s.Default, c.Default},
		"deprecated":           {s.Deprecated, c.Deprecated},
		"readOnly":             {s.ReadOnly, c.ReadOnly},
		"writeOnly":            {s.WriteOnly, c.WriteOnly},
	} {
		if val[0] == val[1] {
			t.Logf("%s has the same memory address.", keyword)
			t.FailNow()
		}
	}

	if slices.Equal(s.Enum, c.Enum) {
		t.Logf("Enum contains values with the same memory address.")
		t.FailNow()
	}

	if slices.Equal(s.Examples, c.Examples) {
		t.Logf("Examples contains values with the same memory address.")
		t.FailNow()
	}
}
