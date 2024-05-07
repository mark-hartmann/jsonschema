package jsonschema_test

import (
	"encoding/json"
	. "jsonschema"
	"reflect"
	"testing"
)

func TestSchema_IsTrue(t *testing.T) {
	schemas := []Schema{
		{},
		{ID: "https://example.com/true.schema.json"},
		{Ref: "https://example.com/true.schema.json"},
		{Defs: map[string]Schema{"true": {}}},
		{Description: "A schema that evaluates to true"},
	}

	for i, schema := range schemas {
		if schema.IsTrue() != (i == 0) {
			t.Errorf("schema at %d is not true but should be: %v", i, schema.String())
		}
	}
}

func TestSchema_IsFalse(t *testing.T) {
	schemas := []Schema{
		{Not: &Schema{}},
		{Ref: "https://example.com/true.schema.json", Not: &Schema{}},
		{ID: "https://example.com/true.schema.json"},
		{Ref: "https://example.com/true.schema.json"},
		{Type: []Type{TypeNull}},
		{Const: 123},
		{PropertyNames: &Schema{MaxLength: ptr(12)}},
	}

	for i, schema := range schemas {
		if schema.IsFalse() != (i <= 1) {
			t.Errorf("schema at %d is not false but should be: %v", i, schema.String())
		}
	}
}

func TestSchema_MarshalJSON(t *testing.T) {
	tests := []struct {
		schema Schema
		json   string
	}{
		{schema: Schema{}, json: "true"},
		{schema: Schema{Not: &Schema{}}, json: "false"},
		{schema: Schema{Const: 123, Not: &Schema{}}, json: "false"},
		{schema: Schema{Const: 123}, json: `{"const":123}`},
		{
			schema: Schema{Ref: "https://example.com/test.schema.json"},
			json:   `{"$ref":"https://example.com/test.schema.json"}`,
		},
	}

	for i, test := range tests {
		b, e := json.Marshal(&test.schema)

		if e != nil {
			t.Logf("unexpected error at %d: %s", i, e)
			t.FailNow()
		}

		if string(b) != test.json {
			t.Logf("test #%d have: %s", i, b)
			t.Logf("test #%d need: %s", i, test.json)
			t.FailNow()
		}
	}
}

func TestTypeSet_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		json   string
		schema Schema
	}{
		{json: "true", schema: Schema{}},
		{json: "false", schema: Schema{Not: &Schema{}}},
		{json: `{"type":"string"}`, schema: Schema{Type: []Type{TypeString}}},
		{json: `{"maximum":123}`, schema: Schema{Maximum: ptr(json.Number("123"))}},
		{json: `{"minItems":10}`, schema: Schema{MinItems: ptr(10)}},
		{
			json:   `{"$ref":"https://example.com/test.schema.json"}`,
			schema: Schema{Ref: "https://example.com/test.schema.json"},
		},
		// Numbers are converted to float64 before being written to an any field
		{json: `{"const":123,"not":{}}`, schema: Schema{Const: float64(123), Not: &Schema{}}},
	}

	for i, test := range tests {
		var s Schema
		e := json.Unmarshal([]byte(test.json), &s)

		if e != nil {
			t.Logf("unexpected error at %d: %s", i, e)
			t.FailNow()
		}

		if !reflect.DeepEqual(&s, &test.schema) {
			t.Logf("test #%d have: %#v", i, s)
			t.Logf("test #%d need: %#v", i, test.schema)
			t.FailNow()
		}
	}
}
