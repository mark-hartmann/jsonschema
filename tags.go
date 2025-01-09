package jsonschema

import (
	"slices"
	"strings"
)

type jsonTag []string

func parseJSONTag(tag string) jsonTag {
	return strings.Split(tag, ",")
}

func (o jsonTag) Name() string {
	if len(o) == 0 {
		return ""
	}
	return o[0]
}

func (o jsonTag) Contains(option string) bool {
	if len(o) < 2 {
		return false
	}
	return slices.Contains(o, option)
}
