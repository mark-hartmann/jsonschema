package jsonschema

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/dave/jennifer/jen"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func GenerateType(schema *Schema, file *jen.File) error {
	name, err := deriveName(schema.ID)
	if err != nil {
		return fmt.Errorf("failed to derive name %q: %w", schema.ID, err)
	}

	typeSet := deriveSchemaType(schema)
	if schema.Ref == "" && len(typeSet) == 0 {
		return fmt.Errorf("schema does not refer to a type: %s", schema)
	}

	file.Type().Id(name).Add(generateType(schema)).Line()
	return nil
}

func deriveName(rawId string) (string, error) {
	if rawId == "" {
		return "", fmt.Errorf("id must not be empty")
	}

	uri, err := url.Parse(rawId)
	if err != nil {
		return "", fmt.Errorf("must be a valid URI: %w", err)
	}

	rawName := path.Base(uri.Path)
	if rawName == "." || rawName == "/" {
		return "", errors.New("last element of path is not a file")
	}

	if strings.Contains(rawName, ".") {
		rawParts := strings.Split(rawName, ".")
		rawName = rawParts[0]
	}

	fields := strings.FieldsFunc(rawName, func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	})

	c := cases.Title(language.AmericanEnglish, cases.NoLower)
	for i, v := range fields {
		if i == 0 && unicode.IsNumber(rune(fields[i][0])) {
			return "", errors.New("name must not start with number")
		}

		fields[i] = c.String(v)
	}

	return strings.Join(fields, ""), nil
}

func deriveSchemaType(schema *Schema) TypeSet {
	if len(schema.Type) > 0 {
		return schema.Type
	}
	if schema.Const != nil {
		if st := deriveValueType(schema.Const); st != "" {
			return TypeSet{st}
		}
		return nil
	}

	var types TypeSet
	for _, value := range schema.Enum {
		if derived := deriveValueType(value); derived != "" && !slices.Contains(types, derived) {
			types = append(types, derived)
		}
	}

	return types
}

func deriveValueType(v any) Type {
	if v == nil {
		return TypeNull
	}

	switch v.(type) {
	case bool:
		return TypeBoolean
	case map[string]any:
		return TypeObject
	case []any:
		return TypeArray
	case float64:
		return TypeNumber
	case int:
		return TypeInteger
	case string:
		return TypeString
	default:
		return ""
	}
}

// generateType constructs a Go type. The nullable argument will make it
// a pointer (*)
func generateType(schema *Schema) jen.Code {
	if schema == nil {
		schema = &Schema{}
	}
	types := deriveSchemaType(schema)

	// If no type exists or more than two types exists without one
	// being null: skip for now
	if l := len(types); l == 0 || l > 2 || !slices.Contains(types, TypeNull) {
		jen.Qual("encoding/json", "RawMessage")
	}

	nullIndex := slices.Index(types, TypeNull)
	nullable := nullIndex != -1
	var typ Type
	if !nullable {
		typ = types[0]
	} else {
		// todo: only works with 2 elements
		typ = types[1-nullIndex]
	}

	var c jen.Code
	switch typ {
	case TypeNull:
		c = jen.Struct()
	case TypeBoolean:
		c = jen.Bool()
	case TypeArray:
		c = jen.Index().Add(generateType(schema.Items))
	case TypeNumber:
		c = jen.Qual("encoding/json", "Number")
	case TypeString:
		c = jen.String()
	case TypeInteger:
		c = jen.Int()
	case TypeObject:
		c = generateObject(schema)
	}

	// if not required and the type is not a ref type like array, make
	// it a pointer
	if nullable && types[0] != TypeArray {
		c = jen.Op("*").Add(c)
	}
	return c
}

var caser = cases.Title(language.AmericanEnglish)

func generateObject(schema *Schema) jen.Code {
	var props []string
	for name, _ := range schema.Properties {
		props = append(props, name)
	}
	sort.Strings(props)

	var fields []jen.Code
	for _, name := range props {
		prop := schema.Properties[name]

		var tag string
		if unicode.IsLower(rune(name[0])) {
			tag = name
		}
		if !slices.Contains(schema.Required, name) {
			tag += ",omitempty"
		}

		typ := generateType(&prop)
		stmt := jen.Id(caser.String(name)).Add(typ)
		if tag != "" {
			stmt = stmt.Tag(map[string]string{"json": tag})
		}
		fields = append(fields, stmt)
	}
	return jen.Struct(fields...)
}
