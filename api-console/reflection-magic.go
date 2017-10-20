package apiconsole

import (
	"encoding/json"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"reflect"
	"strings"
	"time"
	"unicode"
)

type Property struct {
	Name        string `yaml:"-"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
}

func MergeWithTypes(ramlTemplate string, values ...interface{}) string {
	types := buildTypes(values...)

	raml := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(strings.Replace(ramlTemplate, "\t", " ", -1)), &raml); err != nil {
		panic(errors.Wrap(err, "Could not parse raml template"))
	}

	if raml["types"] != nil {
		ramlTypes := raml["types"].(map[interface{}]interface{})
		for name, props := range types {
			if ramlTypes[name] == nil {
				ramlTypes[name] = props
			}
		}
	}

	bytes, err := yaml.Marshal(raml)
	if err != nil {
		panic(errors.Wrap(err, "Could not marshal raml file to bytes"))
	}

	return "#%RAML 1.0\n" + string(bytes)
}

func buildTypes(values ...interface{}) map[string]interface{} {
	types := map[string][]Property{}

	for _, value := range values {
		structTypeName(types, reflect.TypeOf(value))
	}

	ramlTypes := make(map[string]interface{})
	for name, props := range types {
		ramlTypes[name] = typeToYaml(props)
	}

	return ramlTypes
}

func typeToYaml(properties []Property) map[string]interface{} {
	if len(properties) == 1 && properties[0].Name == "" {
		return map[string]interface{}{
			"type": properties[0].Type,
		}

	} else {
		pm := map[string]Property{}
		for _, prop := range properties {
			pm[prop.Name] = prop
		}

		return map[string]interface{}{
			"type":       "object",
			"properties": pm,
		}
	}
}

func typeNameOf(types map[string][]Property, t reflect.Type) string {
	if isPrimitiveType(t) {
		return primitiveTypeName(t)
	}

	switch t.Kind() {
	case reflect.Struct:
		return structTypeName(types, t)

	case reflect.Ptr:
		return structTypeName(types, t.Elem())

	case reflect.Slice:
		return structTypeName(types, t.Elem()) + "[]"

	case reflect.Map:
		return "object"
	}

	panic("Can not generate a name for the type: " + t.String())
}

func structTypeName(types map[string][]Property, t reflect.Type) string {
	if t.Kind() != reflect.Struct {
		panic("Type must be struct but was " + t.String())
	}

	typeName := t.Name()
	// build type if not cached
	if types[typeName] == nil {
		// this is to support recursive types
		types[typeName] = []Property{}

		switch t.Kind() {
		case reflect.Struct:
			types[typeName] = buildStructType(types, t)
		}

	}

	return typeName
}

func instantiateType(t reflect.Type) reflect.Value {
	if t.PkgPath() == "time" && t.Name() == "Time" {
		return reflect.ValueOf(time.Now())
	}

	return reflect.New(t).Elem()
}

func buildStructType(types map[string][]Property, t reflect.Type) []Property {
	marshalerType := reflect.TypeOf((*json.Marshaler)(nil)).Elem()
	if t.Implements(marshalerType) {
		return buildStructTypeForJsonMarshaller(t)
	}

	var props []Property
	for idx := 0; idx < t.NumField(); idx++ {
		field := t.Field(idx)

		fieldName := serializedFieldName(field)
		if fieldName != "" {
			// append all fields of embedded structs directly.
			if field.Anonymous {
				props = append(props, buildStructType(types, field.Type)...)
				continue
			}

			props = append(props, Property{
				Name:        fieldName,
				Type:        typeNameOf(types, field.Type),
				Description: field.Tag.Get("desc"),
			})
		}
	}

	return props
}

// Build a struct type from the given json.Marshal type. This works by first creating and initializing
// an instance of the given type, and then by serializing that instance to json, deserializing it to
// a map and build a type description from that map.
func buildStructTypeForJsonMarshaller(t reflect.Type) []Property {
	el := instantiateType(t)
	ma := el.Interface().(json.Marshaler)

	bytes, err := ma.MarshalJSON()
	if err != nil {
		panic(errors.Wrap(err, "Could not serialize to json."))
	}

	var decoded interface{}
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		panic(errors.Wrap(err, "Could not deserialize from json"))
	}

	_, properties := buildTypeFromExample(decoded)
	return properties
}

// Given an example of a type, this method tries to estimate the
func buildTypeFromExample(example interface{}) (string, []Property) {

	t := reflect.TypeOf(example)
	switch {
	case isPrimitiveType(t):
		return primitiveTypeName(t), []Property{{Type: primitiveTypeName(t)}}

	case t.Kind() == reflect.Map:
		properties := []Property{}
		for key, value := range example.(map[string]interface{}) {
			typeName := typeNameOf(nil, reflect.TypeOf(value))
			properties = append(properties, Property{Name: key, Type: typeName})
		}

		return "object", properties

	case t.Kind() == reflect.Ptr:
		return buildTypeFromExample(reflect.ValueOf(t).Elem().Interface())

	default:
		panic("Can not build type from example of type: " + t.Kind().String())
	}
}

// Estimates the name of the field after serialization. This tries to evaluate the
// the json struct-tag in order to simulate json.Marshal.
// If the field should be ignored for serialization, an empty string will be returned.
func serializedFieldName(field reflect.StructField) string {
	name := field.Name
	if name != "" && unicode.IsLower(rune(name[0])) {
		return ""
	}

	jsonTag := field.Tag.Get("json")
	if jsonTag != "" {
		idx := strings.IndexRune(jsonTag, ',')
		if idx < 0 {
			name = jsonTag
		} else {
			name = jsonTag[:idx]
		}

		// just a minus-sign means "no name"
		if name == "-" {
			name = ""
		}
	}

	if field.Type.Kind() == reflect.Ptr {
		name += "?"
	}

	return name
}

func isPrimitiveType(t reflect.Type) bool {
	switch t.Kind() {
	case
		reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.String:

		return true

	default:
		return false
	}
}

// Name of the given primitive type for the raml spec.
func primitiveTypeName(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Bool:
		return "boolean"

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"

	case reflect.Float32, reflect.Float64:
		return "number"

	case reflect.String:
		return "string"

	default:
		panic("not a primitive type")
	}
}
