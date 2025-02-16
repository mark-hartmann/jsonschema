package jsonschema

import (
	"cmp"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"sync"
)

var (
	numZero      = json.Number("0")
	numMinInt    = json.Number(strconv.FormatInt(math.MinInt, 10))
	numMaxInt    = json.Number(strconv.FormatInt(math.MaxInt, 10))
	numMinInt8   = json.Number(strconv.FormatInt(math.MinInt8, 10))
	numMaxInt8   = json.Number(strconv.FormatInt(math.MaxInt8, 10))
	numMinInt16  = json.Number(strconv.FormatInt(math.MinInt16, 10))
	numMaxInt16  = json.Number(strconv.FormatInt(math.MaxInt16, 10))
	numMinInt32  = json.Number(strconv.FormatInt(math.MinInt32, 10))
	numMaxInt32  = json.Number(strconv.FormatInt(math.MaxInt32, 10))
	numMinInt64  = json.Number(strconv.FormatInt(math.MinInt64, 10))
	numMaxInt64  = json.Number(strconv.FormatInt(math.MaxInt64, 10))
	numMaxUint   = json.Number(strconv.FormatUint(math.MaxUint, 10))
	numMaxUint8  = json.Number(strconv.FormatUint(math.MaxUint8, 10))
	numMaxUint16 = json.Number(strconv.FormatUint(math.MaxUint16, 10))
	numMaxUint32 = json.Number(strconv.FormatUint(math.MaxUint32, 10))
	numMaxUint64 = json.Number(strconv.FormatUint(math.MaxUint64, 10))
)

var m = map[reflect.Kind]Schema{
	reflect.Bool:    {Type: TypeSet{TypeBoolean}},
	reflect.String:  {Type: TypeSet{TypeString}},
	reflect.Float32: {Type: TypeSet{TypeNumber}},
	reflect.Float64: {Type: TypeSet{TypeNumber}},
	reflect.Int:     {Type: TypeSet{TypeInteger}, Minimum: &numMinInt, Maximum: &numMaxInt},
	reflect.Int8:    {Type: TypeSet{TypeInteger}, Minimum: &numMinInt8, Maximum: &numMaxInt8},
	reflect.Int16:   {Type: TypeSet{TypeInteger}, Minimum: &numMinInt16, Maximum: &numMaxInt16},
	reflect.Int32:   {Type: TypeSet{TypeInteger}, Minimum: &numMinInt32, Maximum: &numMaxInt32},
	reflect.Int64:   {Type: TypeSet{TypeInteger}, Minimum: &numMinInt64, Maximum: &numMaxInt64},
	reflect.Uint:    {Type: TypeSet{TypeInteger}, Minimum: &numZero, Maximum: &numMaxUint},
	reflect.Uint8:   {Type: TypeSet{TypeInteger}, Minimum: &numZero, Maximum: &numMaxUint8},
	reflect.Uint16:  {Type: TypeSet{TypeInteger}, Minimum: &numZero, Maximum: &numMaxUint16},
	reflect.Uint32:  {Type: TypeSet{TypeInteger}, Minimum: &numZero, Maximum: &numMaxUint32},
	reflect.Uint64:  {Type: TypeSet{TypeInteger}, Minimum: &numZero, Maximum: &numMaxUint64},
}

// TypeRepository manages known schema definitions for Go types and allow the
// saving of new schemas as well as the loading and referencing of known schemas.
type TypeRepository interface {
	Load(t reflect.Type) (*Schema, bool)
	Store(t reflect.Type, schema *Schema, inline bool) bool
	Ref(t reflect.Type) *Schema
}

// localFinalizer allows implementations of the TypeRepository interface to edit the
// schema after it has been completely generated - for example, to add schemas to the
// $defs map.
type localFinalizer interface {
	// Finalize is called by FromGoType with all previously referenced types. The
	// referenced types are tracked independently of the used type repository.
	Finalize([]reflect.Type, *Schema)
}

type GoTypeConfig struct {
	Types TypeRepository

	// RefTypesNotNullable controls whether Go reference types are implicitly nullable
	// or must be provided via an empty instance. If false, a TypeSet containing TypeNull
	// is generated, otherwise it is just TypeArray or TypeObject. This option has no
	// effect on pointers to reference types.
	RefTypesNotNullable bool
}

type typeEntry struct {
	t reflect.Type
	s *Schema

	name   string
	inline bool
}

type typeRegistry struct {
	sync.Mutex
	types  map[reflect.Type]typeEntry
	nameFn func(t reflect.Type) string
}

func (r *typeRegistry) loadTypeEntry(t reflect.Type) (typeEntry, bool) {
	r.Lock()
	e, ok := r.types[t]
	r.Unlock()
	if !ok {
		return typeEntry{}, false
	}
	return e, ok
}

func (r *typeRegistry) Load(t reflect.Type) (*Schema, bool) {
	e, ok := r.loadTypeEntry(t)
	if !ok {
		return nil, false
	}
	return e.s, ok
}

func (r *typeRegistry) Store(t reflect.Type, schema *Schema, inline bool) bool {
	r.Lock()
	_, ok := r.types[t]
	if !ok {
		r.types[t] = typeEntry{
			s:      schema,
			t:      t,
			name:   r.nameFn(t),
			inline: inline,
		}
	}
	r.Unlock()
	return !ok
}

func (r *typeRegistry) Ref(t reflect.Type) (rs *Schema) {
	r.Lock()
	if e, ok := r.types[t]; ok {
		if e.inline {
			rs = e.s
		} else {
			rs = &Schema{Ref: "#/$defs/" + e.name}
		}
	}
	r.Unlock()
	return
}

func (r *typeRegistry) Finalize(types []reflect.Type, schema *Schema) {
	if len(types) == 0 {
		return
	}
	if schema.Defs == nil {
		schema.Defs = make(map[string]Schema)
	}

	schema.Defs = make(map[string]Schema)
	for i := range types {
		e, _ := r.loadTypeEntry(types[i])
		if e.inline {
			continue
		}
		schema.Defs[e.name] = *e.s
	}
}

// NewSimpleTypeRepository returns a reusable, in-memory implementation of TypeRepository
// that is safe for concurrent use with FromGoType. Referenced schemas are added to the
// $defs object of the generated Schema.
func NewSimpleTypeRepository() TypeRepository {
	return &typeRegistry{
		types:  make(map[reflect.Type]typeEntry),
		nameFn: func(t reflect.Type) string { return t.Name() },
	}
}

// pkgToCamel converts a go package name and path to a camelCase string
func pkgToCamel(s string) string {
	parts := strings.Split(strings.Replace(s, "_", "/", -1), "/")
	cc := strings.ToLower(parts[0])
	for _, word := range parts[1:] {
		cc += string(unicode.ToUpper(rune(word[0]))) + strings.ToLower(word[1:])
	}
	return cc
}

// trackingTypeRepo is a specialized implementation of TypeRepository that wraps the
// actual implementation and tracks the referenced Go types.
type trackingTypeRepo struct {
	TypeRepository
	tracked map[reflect.Type]struct{}
}

func (r *trackingTypeRepo) Ref(t reflect.Type) *Schema {
	var s *Schema
	if s = r.TypeRepository.Ref(t); s != nil {
		r.tracked[t] = struct{}{}
	}
	return s
}

func newTrackingRepo(repository TypeRepository) *trackingTypeRepo {
	return &trackingTypeRepo{
		TypeRepository: repository,
		tracked:        make(map[reflect.Type]struct{}),
	}
}

func FromGoType(t reflect.Type, opts GoTypeConfig) (*Schema, error) {
	if t == nil || (t.Kind() == reflect.Interface && t.NumMethod() == 0) {
		return &True, nil
	}

	if opts.Types == nil {
		opts.Types = NewSimpleTypeRepository()
	}
	types, trackingRepo := opts.Types, newTrackingRepo(opts.Types)
	opts.Types = trackingRepo

	s, err := fromGoType(t, opts)
	if err != nil {
		return nil, fmt.Errorf("schema.FromGoType: %w", err)
	}

	if v, ok := types.(localFinalizer); ok {
		trackedTypes := make([]reflect.Type, 0, len(trackingRepo.tracked))
		for t := range trackingRepo.tracked {
			trackedTypes = append(trackedTypes, t)
		}
		v.Finalize(trackedTypes, s)
	}
	return s, nil
}

func newTyped(t Type, nullable bool) *Schema {
	s := Schema{}
	s.Type = TypeSet{t}
	if nullable {
		s.Type = append(s.Type, TypeNull)
	}
	return &s
}

func isRefType(t reflect.Type) bool {
	return t.Kind() == reflect.Map || t.Kind() == reflect.Array || t.Kind() == reflect.Slice
}

func fromGoType(t reflect.Type, opts GoTypeConfig) (*Schema, error) {
	schema, defined, defs := &Schema{}, false, opts.Types

	if _, found := defs.Load(t); found {
		return defs.Ref(t), nil
	} else if t.PkgPath() != "" && t.Name() != "" {
		// It's a defined type, we can store the empty schema and reference it down
		// the line if necessary. If further calls to fromGoType encounter a defined
		// type that has already been marked as known, a referring schema can be
		// returned.
		defs.Store(t, schema, false)
		defined = true
	}

	if t.Kind() == reflect.Ptr {
		nullable := !isRefType(t.Elem())
		underlying, err := fromGoType(t.Elem(), opts)
		if err != nil {
			return nil, err
		}

		*schema = *underlying
		if nullable {
			if underlying.Ref != "" {
				*schema = Schema{OneOf: []Schema{*underlying, {Type: TypeSet{TypeNull}}}}
			} else {
				if !slices.Contains(schema.Type, TypeNull) {
					schema.Type = append(schema.Type, TypeNull)
				}
			}
		}

		if !defined {
			return schema, nil
		}
		return defs.Ref(t), nil
	}

	// if primitive, we can return early because they are predefined.
	if s, ok := m[t.Kind()]; ok {
		*schema = s
		if defined {
			return defs.Ref(t), nil
		}
		return schema, nil
	}

	var (
		s   *Schema
		err error
	)

	switch t.Kind() {
	case reflect.Array, reflect.Slice:
		s, err = arrType(t, opts)
	case reflect.Struct:
		s, err = structType(t, opts)
	case reflect.Map:
		s, err = mapType(t, opts)
	case reflect.Interface:
		if t.NumMethod() == 0 {
			return &True, nil
		}
		fallthrough
	default:
		return nil, fmt.Errorf("cannot map Go type %s", t)
	}

	if err != nil {
		return nil, err
	}
	*schema = *s
	if defined {
		return defs.Ref(t), nil
	}
	return s, nil

}

type field struct {
	name       string
	required   bool
	requiredIf bool
	named      bool
	typ        reflect.Type
	tag        jsonTag
	index      []int
	quoted     bool
}

func unexportedOrEmbeddedNonStruct(sf reflect.StructField) bool {
	if sf.Anonymous {
		t := sf.Type
		for t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		if !sf.IsExported() && t.Kind() != reflect.Struct {
			return true
		}
	} else if !sf.IsExported() {
		return true
	}
	return false
}

func typeFields(t reflect.Type) []field {
	// If a type has already been visited, its fields are already inlined, any
	// additional inclusion of fields of the same type would be removed in the
	// end anyway.
	visited := map[reflect.Type]bool{}

	next := []field{{typ: t}}
	var current, fields []field
	for len(next) > 0 {
		// Move fields from queue to current and clear queue
		current, next = next, current[:0]
		for _, cf := range current {
			if visited[cf.typ] {
				continue
			}
			visited[cf.typ] = true

			for i := 0; i < cf.typ.NumField(); i++ {
				sf := cf.typ.Field(i)
				if unexportedOrEmbeddedNonStruct(sf) {
					continue
				}

				// Unwrap the type if it is a non-defined pointer.
				sft := sf.Type
				if sft.Name() == "" && sft.Kind() == reflect.Pointer {
					sft = sft.Elem()
				}

				var tag jsonTag
				if rawTag := sf.Tag.Get("json"); rawTag != "-" {
					tag = parseJSONTag(sf.Tag.Get("json"))
				} else {
					continue
				}

				name := tag.Name()

				index := make([]int, len(cf.index)+1)
				copy(index, cf.index)
				index[len(cf.index)] = i

				// Embedded structs without json name are queued for further iterations.
				if tag.Name() == "" && sf.Anonymous && sft.Kind() == reflect.Struct {
					next = append(next, field{
						name:       sft.Name(),
						typ:        sft,
						index:      index,
						requiredIf: sf.Type.Kind() == reflect.Ptr,
					})
					continue
				}

				if tag.Name() == "" {
					name = sf.Name
				}

				// Only strings, floats, integers, and booleans can be quoted.
				quoted := false
				if tag.Contains("string") {
					switch sft.Kind() {
					case reflect.Bool,
						reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
						reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
						reflect.Float32, reflect.Float64,
						reflect.String:
						quoted = true
					}
				}

				if sf.Type.Kind() == reflect.Ptr {
					sft = reflect.PointerTo(sft)
				}

				fields = append(fields, field{
					name:       name,
					typ:        sft,
					tag:        tag,
					named:      tag.Name() != "",
					required:   !tag.Contains("omitempty"),
					requiredIf: cf.requiredIf,
					index:      index,
					quoted:     quoted,
				})
			}
		}
	}

	// Sort fields in order of name, order of field index length and if
	// they are tagged.
	slices.SortFunc(fields, cmpFields)

	// Delete all fields that are hidden by the Go rules for embedded
	// fields, except that fields with JSON tags are promoted. Loop over
	// names; for each name, delete hidden fields by choosing the one
	// dominant field that survives.
	out := fields[:0]
	for advance, i := 0, 0; i < len(fields); i += advance {
		// One iteration per name.
		// Find the sequence of fields with the name of this first field.
		fi := fields[i]
		name := fi.name
		for advance = 1; i+advance < len(fields); advance++ {
			fj := fields[i+advance]
			if fj.name != name {
				break
			}
		}
		if advance == 1 { // Only one field with this name
			out = append(out, fi)
			continue
		}
		dominant, ok := dominantField(fields[i : i+advance])
		if ok {
			out = append(out, dominant)
		}
	}

	fields = out
	slices.SortFunc(fields, cmpFieldIndexes)
	return fields
}

func cmpFieldIndexes(a, b field) int {
	return slices.Compare(a.index, b.index)
}

func cmpFields(a, b field) int {
	// If the name is not equal, sort alphabetically
	if a.name != b.name {
		return cmp.Compare(a.name, b.name)
	}
	// If the fields have different nesting depths, choose the less
	// nested one.
	if len(a.index) != len(b.index) {
		return cmp.Compare(len(a.index), len(b.index))
	}
	// If names and nesting depths are equal and one of the fields is
	// name tagged, choose the tagged field.
	if a.named != b.named {
		if a.named {
			return -1
		}
		return 1
	}

	// The index sequence decides if everything is the same; the
	// lower the positions, the nearer it is to the root top level.
	return slices.Compare(a.index, b.index)
}

// dominantField looks through the fields, all of which are known to
// have the same name, to find the single field that dominates the
// others using Go's embedding rules, modified by the presence of
// JSON tags. If there are multiple top-level fields, the boolean
// will be false: This condition is an error in Go and we skip all
// the fields.
func dominantField(fields []field) (field, bool) {
	// The fields are sorted in increasing index-length order, then by presence of tag.
	// That means that the first field is the dominant one. We need only check
	// for error cases: two fields at top level, either both tagged or neither tagged.
	if len(fields) > 1 && len(fields[0].index) == len(fields[1].index) && fields[0].named == fields[1].named {
		return field{}, false
	}
	return fields[0], true
}

var (
	patternSignedInt   = regexp.MustCompile(`^-?(0|[1-9]\d*)$`)
	patternUnsignedInt = regexp.MustCompile(`^(0|[1-9]\d*)$`)
	patternFractional  = regexp.MustCompile(`^-?(0|[1-9]\d*)(\.\d+)?$`)
)

func patternSchema(regexp *regexp.Regexp) *Schema {
	return &Schema{
		Type:    TypeSet{TypeString},
		Pattern: ptr(regexp.String()),
	}
}

func structType(t reflect.Type, opts GoTypeConfig) (*Schema, error) {
	s := &Schema{Type: TypeSet{TypeObject}}
	s.AdditionalProperties = &False

	fields := typeFields(t)
	properties := make(map[string]Schema, len(fields))

	var hasDependent bool
	for i := 0; i < len(fields); i++ {
		x := fields[i]

		var (
			fs  *Schema
			err error
		)
		if x.quoted {
			var nullable = false
			if x.typ.Kind() == reflect.Ptr {
				nullable = true
				for x.typ.Kind() == reflect.Ptr {
					x.typ = x.typ.Elem()
				}
			}

			switch x.typ.Kind() {
			case reflect.Bool:
				fs = &Schema{Enum: []any{"true", "false"}}
				if nullable {
					fs = &Schema{OneOf: []Schema{*fs, {Type: []Type{TypeNull}}}}
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				fs = patternSchema(patternSignedInt)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				fs = patternSchema(patternUnsignedInt)
			case reflect.Float32, reflect.Float64:
				fs = patternSchema(patternFractional)
			default:
			}

			// add null type if the schema has a type(set)
			if fs != nil && len(fs.Type) > 0 && nullable && !slices.Contains(fs.Type, TypeNull) {
				fs.Type = append(fs.Type, TypeNull)
			}
		} else {
			fs, err = fromGoType(x.typ, opts)
		}
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", x.name, err)
		}

		properties[x.name] = *fs
		if x.required && !x.requiredIf {
			s.Required = append(s.Required, x.name)
		}

		if !hasDependent && x.requiredIf {
			hasDependent = true
		}
	}

	if len(properties) > 0 {
		s.Properties = properties
	}

	// Build dependent required map
	if hasDependent {
		maxDepth := 1
		for i := 0; i < len(fields); i++ {
			if len(fields[i].index) > maxDepth {
				maxDepth++
			}
		}

		layers := make([][]int, maxDepth)
		for i := 0; i < len(fields); i++ {
			j := len(fields[i].index) - 1
			layers[j] = append(layers[j], i)
		}
		var req, opt []string
		for i := 0; i < len(layers); i++ {
			for j := 0; j < len(layers[i]); j++ {
				f := fields[layers[i][j]]
				if !f.requiredIf {
					continue
				}
				if f.required {
					req = append(req, f.name)
				} else {
					opt = append(opt, f.name)
				}
			}
		}

		if len(req) > 0 && len(opt) > 0 {
			s.DependentRequired = buildDependentRequired(req, opt)
		}
	}

	return s, nil
}

func buildDependentRequired(required, options []string) map[string][]string {
	result := make(map[string][]string)
	for i, key := range required {
		for j, value := range required {
			if i != j { // Avoid adding self-references
				result[key] = append(result[key], value)
			}
		}
	}
	for _, key := range options {
		result[key] = append([]string{}, required...)
	}
	return result
}

func arrType(t reflect.Type, opts GoTypeConfig) (*Schema, error) {
	s := newTyped(TypeArray, !opts.RefTypesNotNullable)
	if t.Kind() == reflect.Array {
		s.MaxItems = ptr(t.Len())
	}

	var err error
	if s.Items, err = fromGoType(t.Elem(), opts); err != nil {
		return nil, fmt.Errorf("invalid array element type: %w", err)
	}
	return s, nil
}

func mapType(t reflect.Type, opts GoTypeConfig) (*Schema, error) {
	keyType, valType := t.Key(), t.Elem()
	if keyType.Kind() == reflect.String {
		s := newTyped(TypeObject, !opts.RefTypesNotNullable)
		var err error
		if s.AdditionalProperties, err = fromGoType(valType, opts); err != nil {
			return nil, fmt.Errorf("invalid map value: %w", err)
		}
		return s, nil
	}

	ks, err := fromGoType(keyType, opts)
	if err != nil {
		return nil, fmt.Errorf("invalid map key: %w", err)
	}
	vs, err := fromGoType(valType, opts)
	if err != nil {
		return nil, fmt.Errorf("invalid map value: %w", err)
	}

	s := &Schema{
		Type: TypeSet{TypeObject, TypeNull},
		Properties: map[string]Schema{
			"keys": {
				Type:        TypeSet{TypeArray},
				Items:       ks,
				UniqueItems: ptr(true),
			},
			"values": {
				Type:  TypeSet{TypeArray},
				Items: vs,
			},
		},
		Required:             []string{"keys", "values"},
		AdditionalProperties: &False,
	}
	return s, nil
}

func ptr[T any](v T) *T {
	return &v
}
