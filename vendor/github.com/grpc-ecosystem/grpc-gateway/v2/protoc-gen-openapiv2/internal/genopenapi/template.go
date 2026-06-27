package genopenapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/textproto"
	"os"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/internal/casing"
	"github.com/grpc-ecosystem/grpc-gateway/v2/internal/descriptor"
	openapi_options "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/genproto/googleapis/api/visibility"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/structpb"
)

// The OpenAPI specification does not allow for more than one endpoint with the same HTTP method and path.
// This prevents multiple gRPC service methods from sharing the same stripped version of the path and method.
// For example: `GET /v1/{name=organizations/*}/roles` and `GET /v1/{name=users/*}/roles` both get stripped to `GET /v1/{name}/roles`.
// We must make the URL unique by adding a suffix and an incrementing index to each path parameter
// to differentiate the endpoints.
// Since path parameter names do not affect the request contents (i.e. they're replaced in the path)
// this will be hidden from the real grpc gateway consumer.
const pathParamUniqueSuffixDeliminator = "_"

const paragraphDeliminator = "\n\n"

// wktSchemas are the schemas of well-known-types.
// The schemas must match with the behavior of the JSON unmarshaler in
// https://github.com/protocolbuffers/protobuf-go/blob/v1.25.0/encoding/protojson/well_known_types.go
var wktSchemas = map[string]schemaCore{
	".google.protobuf.FieldMask": {
		Type: "string",
	},
	".google.protobuf.Timestamp": {
		Type:   "string",
		Format: "date-time",
	},
	".google.protobuf.Duration": {
		Type: "string",
	},
	".google.protobuf.StringValue": {
		Type: "string",
	},
	".google.protobuf.BytesValue": {
		Type:   "string",
		Format: "byte",
	},
	".google.protobuf.Int32Value": {
		Type:   "integer",
		Format: "int32",
	},
	".google.protobuf.UInt32Value": {
		Type:   "integer",
		Format: "int64",
	},
	".google.protobuf.Int64Value": {
		Type:   "string",
		Format: "int64",
	},
	".google.protobuf.UInt64Value": {
		Type:   "string",
		Format: "uint64",
	},
	".google.protobuf.FloatValue": {
		Type:   "number",
		Format: "float",
	},
	".google.protobuf.DoubleValue": {
		Type:   "number",
		Format: "double",
	},
	".google.protobuf.BoolValue": {
		Type: "boolean",
	},
	".google.protobuf.Empty": {
		Type: "object",
	},
	".google.protobuf.Struct": {
		Type: "object",
	},
	".google.protobuf.Value": {},
	".google.protobuf.ListValue": {
		Type: "array",
		Items: (*openapiItemsObject)(&openapiSchemaObject{
			schemaCore: schemaCore{
				Type: "object",
			},
		}),
	},
	".google.protobuf.NullValue": {
		Type: "string",
	},
}

func listEnumNames(reg *descriptor.Registry, enum *descriptor.Enum) interface{} {
	var names []string
	for _, value := range enum.GetValue() {
		if !isVisible(getEnumValueVisibilityOption(value), reg) {
			continue
		}
		if reg.GetOmitEnumDefaultValue() && value.GetNumber() == 0 {
			continue
		}
		names = append(names, value.GetName())
	}

	if len(names) > 0 {
		return names
	}

	return nil
}

func listEnumNumbers(reg *descriptor.Registry, enum *descriptor.Enum) interface{} {
	var numbers []int
	for _, value := range enum.GetValue() {
		if reg.GetOmitEnumDefaultValue() && value.GetNumber() == 0 {
			continue
		}
		if !isVisible(getEnumValueVisibilityOption(value), reg) {
			continue
		}
		numbers = append(numbers, int(value.GetNumber()))
	}

	if len(numbers) > 0 {
		return numbers
	}

	return nil
}

func getEnumDefault(reg *descriptor.Registry, enum *descriptor.Enum) interface{} {
	if !reg.GetOmitEnumDefaultValue() {
		for _, value := range enum.GetValue() {
			if value.GetNumber() == 0 {
				if !isVisible(getEnumValueVisibilityOption(value), reg) {
					return nil
				}
				return value.GetName()
			}
		}
	}
	return nil
}

func getEnumDefaultNumber(reg *descriptor.Registry, enum *descriptor.Enum) interface{} {
	if !reg.GetOmitEnumDefaultValue() {
		for _, value := range enum.GetValue() {
			if value.GetNumber() == 0 {
				return int(value.GetNumber())
			}
		}
	}
	return nil
}

// messageToQueryParameters converts a message to a list of OpenAPI query parameters.
func messageToQueryParameters(message *descriptor.Message, reg *descriptor.Registry, pathParams []descriptor.Parameter, body *descriptor.Body, httpMethod string) (params []openapiParameterObject, err error) {
	for _, field := range message.Fields {
		// When body is set to oneof field, we want to skip other fields in the oneof group.
		if isBodySameOneOf(body, field) {
			continue
		}

		if !isVisible(getFieldVisibilityOption(field), reg) {
			continue
		}
		if reg.GetAllowPatchFeature() && field.GetTypeName() == ".google.protobuf.FieldMask" && field.GetName() == "update_mask" && httpMethod == "PATCH" && len(body.FieldPath) != 0 {
			continue
		}

		p, err := queryParams(message, field, "", reg, pathParams, body, reg.GetRecursiveDepth())
		if err != nil {
			return nil, err
		}
		params = append(params, p...)
	}
	return params, nil
}

func isBodySameOneOf(body *descriptor.Body, field *descriptor.Field) bool {
	if field.OneofIndex == nil {
		return false
	}

	if body == nil || len(body.FieldPath) == 0 {
		return false
	}

	if body.FieldPath[0].Target.OneofIndex == nil {
		return false
	}

	return *body.FieldPath[0].Target.OneofIndex == *field.OneofIndex
}

// queryParams converts a field to a list of OpenAPI query parameters recursively through the use of nestedQueryParams.
func queryParams(message *descriptor.Message, field *descriptor.Field, prefix string, reg *descriptor.Registry, pathParams []descriptor.Parameter, body *descriptor.Body, recursiveCount int) (params []openapiParameterObject, err error) {
	return nestedQueryParams(message, field, prefix, reg, pathParams, body, newCycleChecker(recursiveCount))
}

type cycleChecker struct {
	m     map[string]int
	count int
}

func newCycleChecker(recursive int) *cycleChecker {
	return &cycleChecker{
		m:     make(map[string]int),
		count: recursive,
	}
}

// Check returns whether name is still within recursion
// toleration
func (c *cycleChecker) Check(name string) bool {
	count, ok := c.m[name]
	count += 1
	isCycle := count > c.count

	if isCycle {
		return false
	}

	// provision map entry if not available
	if !ok {
		c.m[name] = 1
		return true
	}

	c.m[name] = count

	return true
}

func (c *cycleChecker) Branch() *cycleChecker {
	copy := &cycleChecker{
		count: c.count,
		m:     make(map[string]int, len(c.m)),
	}

	for k, v := range c.m {
		copy.m[k] = v
	}

	return copy
}

// nestedQueryParams converts a field to a list of OpenAPI query parameters recursively.
// This function is a helper function for queryParams, that keeps track of cyclical message references
// through the use of
//
//	touched map[string]int
//
// If a cycle is discovered, an error is returned, as cyclical data structures are dangerous
// in query parameters.
func nestedQueryParams(message *descriptor.Message, field *descriptor.Field, prefix string, reg *descriptor.Registry, pathParams []descriptor.Parameter, body *descriptor.Body, cycle *cycleChecker) (params []openapiParameterObject, err error) {
	// make sure the parameter is not already listed as a path parameter
	for _, pathParam := range pathParams {
		if pathParam.Target == field {
			return nil, nil
		}
	}
	// make sure the parameter is not already listed as a body parameter
	if body != nil {
		if body.FieldPath == nil {
			return nil, nil
		}
		for _, fieldPath := range body.FieldPath {
			if fieldPath.Target == field {
				return nil, nil
			}
		}
	}
	schema := schemaOfField(field, reg, nil)
	fieldType := field.GetTypeName()
	if message.File != nil {
		comments := fieldProtoComments(reg, message, field)
		if err := updateOpenAPIDataFromComments(reg, &schema, message, comments, false); err != nil {
			return nil, err
		}
	}

	isEnum := field.GetType() == descriptorpb.FieldDescriptorProto_TYPE_ENUM
	items := schema.Items
	if schema.Type != "" || isEnum {
		if schema.Type == "object" {
			location := ""
			if ix := strings.LastIndex(field.Message.FQMN(), "."); ix > 0 {
				location = field.Message.FQMN()[0:ix]
			}
			if m, err := reg.LookupMsg(location, field.GetTypeName()); err == nil {
				if opt := m.GetOptions(); opt != nil && opt.MapEntry != nil && *opt.MapEntry {
					k := m.GetField()[0]
					kType, err := getMapParamKey(k.GetType())
					if err != nil {
						return nil, err
					}
					// This will generate a query in the format map_name[key_type]
					fName := fmt.Sprintf("%s[%s]", *field.Name, kType)
					field.Name = proto.String(fName)
					schema.Type = schema.AdditionalProperties.schemaCore.Type
				}
			}
		}
		if items != nil && (items.Type == "" || items.Type == "object") && !isEnum {
			return nil, nil // TODO: currently, mapping object in query parameter is not supported
		}
		desc := mergeDescription(schema)

		// verify if the field is required
		required := false
		for _, fieldName := range schema.Required {
			if fieldName == reg.FieldName(field) {
				required = true
				break
			}
		}
		// verify if the field is required in message options
		if messageSchema, err := extractSchemaOptionFromMessageDescriptor(message.DescriptorProto); err == nil {
			for _, fieldName := range messageSchema.GetJsonSchema().GetRequired() {
				// Required fields can be field names or json_name values
				if fieldName == field.GetJsonName() || fieldName == field.GetName() {
					required = true
					break
				}
			}
		}

		// verify if the field is deprecated, either via proto or annotation
		protoDeprecated := field.GetOptions().GetDeprecated() && reg.GetEnableFieldDeprecation()
		annotationDeprecated := getFieldConfiguration(reg, field).GetDeprecated()
		deprecated := protoDeprecated || annotationDeprecated

		param := openapiParameterObject{
			Description: desc,
			In:          "query",
			Default:     schema.Default,
			Type:        schema.Type,
			Items:       schema.Items,
			Format:      schema.Format,
			Pattern:     schema.Pattern,
			Required:    required,
			Deprecated:  deprecated,
			UniqueItems: schema.UniqueItems,
			extensions:  schema.extensions,
			Enum:        schema.Enum,
		}
		if param.Type == "array" {
			param.CollectionFormat = "multi"
		}

		param.Name = prefix + reg.FieldName(field)

		if isEnum {
			enum, err := reg.LookupEnum("", fieldType)
			if err != nil {
				return nil, fmt.Errorf("unknown enum type %s", fieldType)
			}
			if items != nil { // array
				param.Items = &openapiItemsObject{
					schemaCore: schemaCore{
						Type: "string",
						Enum: listEnumNames(reg, enum),
					},
				}
				if reg.GetEnumsAsInts() {
					param.Items.Type = "integer"
					param.Items.Enum = listEnumNumbers(reg, enum)
				}
			} else {
				param.Type = "string"
				param.Enum = listEnumNames(reg, enum)
				param.Default = getEnumDefault(reg, enum)
				if reg.GetEnumsAsInts() {
					param.Type = "integer"
					param.Enum = listEnumNumbers(reg, enum)
					param.Default = getEnumDefaultNumber(reg, enum)
				}
			}
			valueComments := enumValueProtoComments(reg, enum)
			if valueComments != "" {
				param.Description = strings.TrimLeft(param.Description+"\n\n "+valueComments, "\n")
			}
		}
		return []openapiParameterObject{param}, nil
	}

	// nested type, recurse
	msg, err := reg.LookupMsg("", fieldType)
	if err != nil {
		return nil, fmt.Errorf("unknown message type %s", fieldType)
	}

	// Check for cyclical message reference:
	if ok := cycle.Check(*msg.Name); !ok {
		return nil, fmt.Errorf("exceeded recursive count (%d) for query parameter %q", cycle.count, fieldType)
	}

	// Construct a new map with the message name so a cycle further down the recursive path can be detected.
	// Do not keep anything in the original touched reference and do not pass that reference along.  This will
	// prevent clobbering adjacent records while recursing.
	touchedOut := cycle.Branch()

	for _, nestedField := range msg.Fields {
		if !isVisible(getFieldVisibilityOption(nestedField), reg) {
			continue
		}

		fieldName := reg.FieldName(field)
		p, err := nestedQueryParams(msg, nestedField, prefix+fieldName+".", reg, pathParams, body, touchedOut)
		if err != nil {
			return nil, err
		}
		params = append(params, p...)
	}
	return params, nil
}

func getMapParamKey(t descriptorpb.FieldDescriptorProto_Type) (string, error) {
	tType, f, ok := primitiveSchema(t)
	if !ok || f == "byte" || f == "float" || f == "double" {
		return "", fmt.Errorf("unsupported type: %q", f)
	}
	return tType, nil
}

// findServicesMessagesAndEnumerations discovers all messages and enums defined in the RPC methods of the service.
func findServicesMessagesAndEnumerations(s []*descriptor.Service, reg *descriptor.Registry, m messageMap, ms messageMap, e enumMap, refs refMap) {
	for _, svc := range s {
		if !isVisible(getServiceVisibilityOption(svc), reg) {
			continue
		}

		for _, meth := range svc.Methods {
			// Request may be fully included in query
			{
				if !isVisible(getMethodVisibilityOption(meth), reg) {
					continue
				}

				// Only process methods with HTTP bindings (exposed via HTTP annotations)
				// This prevents unused message definitions from appearing in the OpenAPI document
				if len(meth.Bindings) == 0 {
					continue
				}

				swgReqName, ok := fullyQualifiedNameToOpenAPIName(meth.RequestType.FQMN(), reg)
				if !ok {
					grpclog.Errorf("couldn't resolve OpenAPI name for FQMN %q", meth.RequestType.FQMN())
					continue
				}
				if _, ok := refs[fmt.Sprintf("#/definitions/%s", swgReqName)]; ok {
					if !skipRenderingRef(meth.RequestType.FQMN()) {
						m[swgReqName] = meth.RequestType
					}
				}
			}

			swgRspName, ok := fullyQualifiedNameToOpenAPIName(meth.ResponseType.FQMN(), reg)
			if !ok && !skipRenderingRef(meth.ResponseType.FQMN()) {
				grpclog.Errorf("couldn't resolve OpenAPI name for FQMN %q", meth.ResponseType.FQMN())
				continue
			}

			findNestedMessagesAndEnumerations(meth.RequestType, reg, m, e)

			if !skipRenderingRef(meth.ResponseType.FQMN()) {
				m[swgRspName] = meth.ResponseType
			}
			findNestedMessagesAndEnumerations(meth.ResponseType, reg, m, e)
		}
	}
}

// findNestedMessagesAndEnumerations those can be generated by the services.
func findNestedMessagesAndEnumerations(message *descriptor.Message, reg *descriptor.Registry, m messageMap, e enumMap) {
	// Iterate over all the fields that
	for _, t := range message.Fields {
		if !isVisible(getFieldVisibilityOption(t), reg) {
			continue
		}

		fieldType := t.GetTypeName()
		// If the type is an empty string then it is a proto primitive
		if fieldType != "" {
			if _, ok := m[fieldType]; !ok {
				msg, err := reg.LookupMsg("", fieldType)
				if err != nil {
					enum, err := reg.LookupEnum("", fieldType)
					if err != nil {
						panic(err)
					}
					e[fieldType] = enum
					continue
				}
				m[fieldType] = msg
				findNestedMessagesAndEnumerations(msg, reg, m, e)
			}
		}
	}
}

// collectReferencedNamesForCache scans services and messages to collect all
// FQMNs/FQENs that will be referenced, WITHOUT using the naming cache.
// This allows us to build the cache with the correct filtered names BEFORE
// any code tries to use it.
func collectReferencedNamesForCache(services []*descriptor.Service, messages []*descriptor.Message, reg *descriptor.Registry) map[string]bool {
	refs := make(map[string]bool)

	// Scan services FIRST so collectNestedTypeFQNs fully traverses
	// message graphs without being short-circuited by pre-populated entries.
	for _, svc := range services {
		if !isVisible(getServiceVisibilityOption(svc), reg) {
			continue
		}
		for _, meth := range svc.Methods {
			if !isVisible(getMethodVisibilityOption(meth), reg) {
				continue
			}
			if len(meth.Bindings) == 0 {
				continue
			}

			// Add method FQN (needed for body:"*" with path params)
			refs[meth.FQMN()] = true

			// Add request/response types
			refs[meth.RequestType.FQMN()] = true
			refs[meth.ResponseType.FQMN()] = true

			// Recursively add nested types
			collectNestedTypeFQNs(meth.RequestType, reg, refs)
			collectNestedTypeFQNs(meth.ResponseType, reg, refs)
		}
	}

	// Add messages from the current file AFTER service scanning.
	// This must come after the service loop's collectNestedTypeFQNs calls,
	// otherwise pre-populated message entries cause the traversal to
	// short-circuit and miss nested types like enums inside referenced messages.
	// We also traverse each message's nested types here because
	// renderMessagesAsDefinition renders ALL messages from the file, not just
	// those reachable from service methods. Without this, cross-package types
	// referenced by non-service messages would be missing from the naming cache.
	for _, msg := range messages {
		refs[msg.FQMN()] = true
		collectNestedTypeFQNs(msg, reg, refs)
	}

	// Add google.rpc.Status if default errors enabled
	if !reg.GetDisableDefaultErrors() {
		refs[".google.rpc.Status"] = true
		// Also add nested types of Status
		if statusMsg, err := reg.LookupMsg("google.rpc", "Status"); err == nil {
			collectNestedTypeFQNs(statusMsg, reg, refs)
		}
	}

	return refs
}

// collectNestedTypeFQNs recursively collects FQMNs/FQENs for all nested types
// of a message. Does NOT use the naming cache.
func collectNestedTypeFQNs(message *descriptor.Message, reg *descriptor.Registry, refs map[string]bool) {
	for _, field := range message.Fields {
		if !isVisible(getFieldVisibilityOption(field), reg) {
			continue
		}
		fieldType := field.GetTypeName()
		if fieldType == "" {
			continue // primitive type
		}
		if refs[fieldType] {
			continue // already visited
		}
		refs[fieldType] = true

		// If it's a message, recurse
		if msg, err := reg.LookupMsg("", fieldType); err == nil {
			collectNestedTypeFQNs(msg, reg, refs)
		}
		// Enums don't have nested types, no recursion needed
	}
}

func skipRenderingRef(refName string) bool {
	_, ok := wktSchemas[refName]
	return ok
}

func renderMessageAsDefinition(msg *descriptor.Message, reg *descriptor.Registry, customRefs refMap, pathParams []descriptor.Parameter) (openapiSchemaObject, error) {
	schema := openapiSchemaObject{
		schemaCore: schemaCore{
			Type: "object",
		},
	}

	if reg.GetGenerateXGoType() && msg.File.GoPkg.Path != "" {
		if schema.extensions == nil {
			schema.extensions = []extension{}
		}
		goTypeName := msg.GetName()

		goTypeName = casing.JSONCamelCase(goTypeName)
		schema.extensions = append(schema.extensions, extension{
			key: "x-go-type",
			value: json.RawMessage(`{
                "import": {
                    "package": "` + msg.File.GoPkg.Path + `"
                },
                "type": "` + goTypeName + `"
            }`),
		})
	}

	msgComments := protoComments(reg, msg.File, msg.Outers, "MessageType", int32(msg.Index))
	if err := updateOpenAPIDataFromComments(reg, &schema, msg, msgComments, false); err != nil {
		return openapiSchemaObject{}, err
	}
	opts, err := getMessageOpenAPIOption(reg, msg)
	if err != nil {
		return openapiSchemaObject{}, err
	}
	if opts != nil {
		protoSchema := openapiSchemaFromProtoSchema(opts, reg, customRefs, msg)

		// Warning: Make sure not to overwrite any fields already set on the schema type.
		schema.ExternalDocs = protoSchema.ExternalDocs
		schema.ReadOnly = protoSchema.ReadOnly
		schema.MultipleOf = protoSchema.MultipleOf
		schema.Maximum = protoSchema.Maximum
		schema.ExclusiveMaximum = protoSchema.ExclusiveMaximum
		schema.Minimum = protoSchema.Minimum
		schema.ExclusiveMinimum = protoSchema.ExclusiveMinimum
		schema.MaxLength = protoSchema.MaxLength
		schema.MinLength = protoSchema.MinLength
		schema.Pattern = protoSchema.Pattern
		schema.Default = protoSchema.Default
		schema.MaxItems = protoSchema.MaxItems
		schema.MinItems = protoSchema.MinItems
		schema.UniqueItems = protoSchema.UniqueItems
		schema.MaxProperties = protoSchema.MaxProperties
		schema.MinProperties = protoSchema.MinProperties
		schema.Required = protoSchema.Required
		schema.XNullable = protoSchema.XNullable
		schema.extensions = protoSchema.extensions
		if protoSchema.schemaCore.Type != "" || protoSchema.schemaCore.Ref != "" {
			schema.schemaCore = protoSchema.schemaCore
		}
		if protoSchema.Title != "" {
			schema.Title = protoSchema.Title
		}
		if protoSchema.Description != "" {
			schema.Description = protoSchema.Description
		}
		if protoSchema.Example != nil {
			schema.Example = protoSchema.Example
		}
	}

	schema.Required = filterOutExcludedFields(schema.Required, pathParams)

	for _, f := range msg.Fields {
		if !isVisible(getFieldVisibilityOption(f), reg) {
			continue
		}

		if shouldExcludeField(f.GetName(), pathParams) {
			continue
		}
		subPathParams := subPathParams(f.GetName(), pathParams)
		fieldSchema, err := renderFieldAsDefinition(f, reg, customRefs, subPathParams)
		if err != nil {
			return openapiSchemaObject{}, err
		}
		comments := fieldProtoComments(reg, msg, f)
		if err := updateOpenAPIDataFromComments(reg, &fieldSchema, f, comments, false); err != nil {
			return openapiSchemaObject{}, err
		}

		if requiredIdx := find(schema.Required, *f.Name); requiredIdx != -1 && reg.GetUseJSONNamesForFields() {
			schema.Required[requiredIdx] = f.GetJsonName()
		}

		if fieldSchema.Required != nil {
			// Only hoist required fields to parent if there are no path params inside this field.
			if len(subPathParams) == 0 {
				schema.Required = getUniqueFields(schema.Required, fieldSchema.Required)
				schema.Required = append(schema.Required, fieldSchema.Required...)
				// To avoid populating both the field schema require and message schema require, unset the field schema require.
				// See issue #2635.
				fieldSchema.Required = nil
			} else {
				// When there are path params, we need to separate field-level required from nested required.
				// The field name itself (if required) should be in parent's required, but nested field names
				// should stay in the nested schema's required.
				fieldName := f.GetName()
				if reg.GetUseJSONNamesForFields() {
					fieldName = f.GetJsonName()
				}
				// Check if the field name is in the fieldSchema.Required (it would be if the field is marked REQUIRED)
				var nestedRequired []string
				fieldIsRequired := false
				for _, req := range fieldSchema.Required {
					if req == fieldName {
						fieldIsRequired = true
					} else {
						nestedRequired = append(nestedRequired, req)
					}
				}
				// Add the field name to parent's required if the field itself is required
				if fieldIsRequired && find(schema.Required, fieldName) == -1 {
					schema.Required = append(schema.Required, fieldName)
				}
				// Keep only the nested required fields in the field schema
				fieldSchema.Required = nestedRequired
			}
		}

		if reg.GetUseAllOfForRefs() {
			if fieldSchema.Ref != "" {
				// Per the JSON Reference syntax: Any members other than "$ref" in a JSON Reference object SHALL be ignored.
				// https://tools.ietf.org/html/draft-pbryan-zyp-json-ref-03#section-3
				// However, use allOf to specify Title/Description/Example/readOnly fields.
				if fieldSchema.Title != "" || fieldSchema.Description != "" || len(fieldSchema.Example) > 0 || fieldSchema.ReadOnly {
					fieldSchema = openapiSchemaObject{
						Title:       fieldSchema.Title,
						Description: fieldSchema.Description,
						schemaCore: schemaCore{
							Example: fieldSchema.Example,
						},
						ReadOnly: fieldSchema.ReadOnly,
						AllOf:    []allOfEntry{{Ref: fieldSchema.Ref}},
					}
				} else {
					fieldSchema = openapiSchemaObject{schemaCore: schemaCore{Ref: fieldSchema.Ref}}
				}
			}
		}

		kv := keyVal{Value: fieldSchema}
		kv.Key = reg.FieldName(f)
		if schema.Properties == nil {
			schema.Properties = &openapiSchemaObjectProperties{}
		}
		*schema.Properties = append(*schema.Properties, kv)
	}

	if msg.FQMN() == ".google.protobuf.Any" {
		transformAnyForJSON(&schema, reg.GetUseJSONNamesForFields())
	}

	return schema, nil
}

func renderFieldAsDefinition(f *descriptor.Field, reg *descriptor.Registry, refs refMap, pathParams []descriptor.Parameter) (openapiSchemaObject, error) {
	if len(pathParams) == 0 {
		return schemaOfField(f, reg, refs), nil
	}
	location := ""
	if ix := strings.LastIndex(f.Message.FQMN(), "."); ix > 0 {
		location = f.Message.FQMN()[0:ix]
	}
	msg, err := reg.LookupMsg(location, f.GetTypeName())
	if err != nil {
		return openapiSchemaObject{}, err
	}
	schema, err := renderMessageAsDefinition(msg, reg, refs, pathParams)
	if err != nil {
		return openapiSchemaObject{}, err
	}
	comments := fieldProtoComments(reg, f.Message, f)
	if len(comments) > 0 {
		// Use title and description from field instead of nested message if present.
		paragraphs := strings.Split(comments, paragraphDeliminator)
		firstParagraph := strings.TrimSpace(paragraphs[0])
		if !strings.Contains(firstParagraph, "\n") {
			schema.Title = firstParagraph
			schema.Description = strings.TrimSpace(strings.Join(paragraphs[1:], paragraphDeliminator))
		} else {
			schema.Description = strings.TrimSpace(comments)
		}
	}

	// to handle case where path param is present inside the field of descriptorpb.FieldDescriptorProto_TYPE_MESSAGE type
	// it still needs to consider the behaviour of the field which was being done by schemaOfField() in case there are no path params
	if j, err := getFieldBehaviorOption(reg, f); err == nil {
		updateSwaggerObjectFromFieldBehavior(&schema, j, reg, f)
	}

	return schema, nil
}

// transformAnyForJSON should be called when the schema object represents a google.protobuf.Any, and will replace the
// Properties slice with a single value for '@type'. We mutate the incorrectly named field so that we inherit the same
// documentation as specified on the original field in the protobuf descriptors.
func transformAnyForJSON(schema *openapiSchemaObject, useJSONNames bool) {
	var typeFieldName string
	if useJSONNames {
		typeFieldName = "typeUrl"
	} else {
		typeFieldName = "type_url"
	}

	for _, property := range *schema.Properties {
		if property.Key == typeFieldName {
			schema.AdditionalProperties = &openapiSchemaObject{}
			schema.Properties = &openapiSchemaObjectProperties{keyVal{
				Key:   "@type",
				Value: property.Value,
			}}
			break
		}
	}
}

func renderMessagesAsDefinition(messages messageMap, d openapiDefinitionsObject, reg *descriptor.Registry, customRefs refMap, pathParams []descriptor.Parameter) error {
	for name, msg := range messages {
		swgName, ok := fullyQualifiedNameToOpenAPIName(msg.FQMN(), reg)
		if !ok {
			return fmt.Errorf("can't resolve OpenAPI name from %q", msg.FQMN())
		}
		if skipRenderingRef(name) {
			continue
		}

		if opt := msg.GetOptions(); opt != nil && opt.MapEntry != nil && *opt.MapEntry {
			continue
		}
		var err error
		d[swgName], err = renderMessageAsDefinition(msg, reg, customRefs, pathParams)
		if err != nil {
			return err
		}
	}
	return nil
}

// isVisible checks if a field/RPC is visible based on the visibility restriction
// combined with the `visibility_restriction_selectors`.
// Elements with an overlap on `visibility_restriction_selectors` are visible, those without are not visible.
// Elements without `google.api.VisibilityRule` annotations entirely are always visible.
func isVisible(r *visibility.VisibilityRule, reg *descriptor.Registry) bool {
	if r == nil {
		return true
	}

	restrictions := strings.Split(strings.TrimSpace(r.Restriction), ",")
	// No restrictions results in the element always being visible
	if len(restrictions) == 0 {
		return true
	}

	for _, restriction := range restrictions {
		if reg.GetVisibilityRestrictionSelectors()[strings.TrimSpace(restriction)] {
			return true
		}
	}

	return false
}

func shouldExcludeField(name string, excluded []descriptor.Parameter) bool {
	for _, p := range excluded {
		if len(p.FieldPath) == 1 && name == p.FieldPath[0].Name {
			return true
		}
	}
	return false
}

func filterOutExcludedFields(fields []string, excluded []descriptor.Parameter) []string {
	var filtered []string
	for _, f := range fields {
		if !shouldExcludeField(f, excluded) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// schemaOfFieldBase returns a base Schema Object for a protobuf field.
func schemaOfFieldBase(f *descriptor.Field, reg *descriptor.Registry, refs refMap) openapiSchemaObject {
	const (
		singular = 0
		array    = 1
		object   = 2
	)
	var (
		core      schemaCore
		aggregate int
	)

	fd := f.FieldDescriptorProto
	location := ""
	if ix := strings.LastIndex(f.Message.FQMN(), "."); ix > 0 {
		location = f.Message.FQMN()[0:ix]
	}
	if m, err := reg.LookupMsg(location, f.GetTypeName()); err == nil {
		if opt := m.GetOptions(); opt != nil && opt.MapEntry != nil && *opt.MapEntry {
			fd = m.GetField()[1]
			aggregate = object
		}
	}
	if fd.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
		aggregate = array
	}

	var props *openapiSchemaObjectProperties

	switch ft := fd.GetType(); ft {
	case descriptorpb.FieldDescriptorProto_TYPE_ENUM, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_TYPE_GROUP:
		if wktSchema, ok := wktSchemas[fd.GetTypeName()]; ok {
			core = wktSchema
			if fd.GetTypeName() == ".google.protobuf.Empty" {
				props = &openapiSchemaObjectProperties{}
			}
		} else {
			swgRef, ok := fullyQualifiedNameToOpenAPIName(fd.GetTypeName(), reg)
			if !ok {
				panic(fmt.Sprintf("can't resolve OpenAPI ref from typename %q", fd.GetTypeName()))
			}
			core = schemaCore{
				Ref: "#/definitions/" + swgRef,
			}
			if refs != nil {
				refs[fd.GetTypeName()] = struct{}{}
			}
		}
	default:
		ftype, format, ok := primitiveSchema(ft)
		if ok {
			core = schemaCore{Type: ftype, Format: format}
		} else {
			core = schemaCore{Type: ft.String(), Format: "UNKNOWN"}
		}
	}

	var ret openapiSchemaObject

	switch aggregate {
	case array:
		// Only set core.Type = "object" for MESSAGE types with $ref if the flag is not set.
		// When omitArrayItemTypeWhenRefSibling is true, we omit "type: object" to avoid
		// no-$ref-siblings violations in OpenAPI v2, since $ref already implies the type is object.
		if !reg.GetOmitArrayItemTypeWhenRefSibling() {
			if _, ok := wktSchemas[fd.GetTypeName()]; !ok && fd.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
				core.Type = "object"
			}
		}
		ret = openapiSchemaObject{
			schemaCore: schemaCore{
				Type:  "array",
				Items: (*openapiItemsObject)(&openapiSchemaObject{schemaCore: core}),
			},
		}
	case object:
		ret = openapiSchemaObject{
			schemaCore: schemaCore{
				Type: "object",
			},
			AdditionalProperties: &openapiSchemaObject{Properties: props, schemaCore: core},
		}
	default:
		ret = openapiSchemaObject{
			schemaCore: core,
			Properties: props,
		}
	}
	return ret
}

// schemaOfField returns a OpenAPI Schema Object for a protobuf field.
func schemaOfField(f *descriptor.Field, reg *descriptor.Registry, refs refMap) openapiSchemaObject {
	ret := schemaOfFieldBase(f, reg, refs)

	if j, err := getFieldOpenAPIOption(reg, f); err == nil {
		updateswaggerObjectFromJSONSchema(&ret, j, reg, f)
	}

	if j, err := getFieldBehaviorOption(reg, f); err == nil {
		updateSwaggerObjectFromFieldBehavior(&ret, j, reg, f)
	}

	for i, required := range ret.Required {
		if required == f.GetName() {
			ret.Required[i] = reg.FieldName(f)
		}
	}

	slices.Sort(ret.Required)
	ret.Required = slices.Compact(ret.Required)

	if reg.GetProto3OptionalNullable() && f.GetProto3Optional() {
		ret.XNullable = true
	}

	return ret
}

// primitiveSchema returns a pair of "Type" and "Format" in JSON Schema for
// the given primitive field type.
// The last return parameter is true iff the field type is actually primitive.
func primitiveSchema(t descriptorpb.FieldDescriptorProto_Type) (ftype, format string, ok bool) {
	switch t {
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		return "number", "double", true
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		return "number", "float", true
	case descriptorpb.FieldDescriptorProto_TYPE_INT64:
		return "string", "int64", true
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64:
		// 64bit integer types are marshaled as string in the default JSONPb marshaler.
		// TODO(yugui) Add an option to declare 64bit integers as int64.
		//
		// NOTE: uint64 is not a predefined format of integer type in OpenAPI spec.
		// So we cannot expect that uint64 is commonly supported by OpenAPI processor.
		return "string", "uint64", true
	case descriptorpb.FieldDescriptorProto_TYPE_INT32:
		return "integer", "int32", true
	case descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		// Ditto.
		return "string", "uint64", true
	case descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		// Ditto.
		return "integer", "int64", true
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		// NOTE: in OpenAPI specification, format should be empty on boolean type
		return "boolean", "", true
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		// NOTE: in OpenAPI specification, can be empty on string type
		// see: https://swagger.io/specification/v2/#data-types
		return "string", "", true
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return "string", "byte", true
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32:
		// Ditto.
		return "integer", "int64", true
	case descriptorpb.FieldDescriptorProto_TYPE_SFIXED32:
		return "integer", "int32", true
	case descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		return "string", "int64", true
	case descriptorpb.FieldDescriptorProto_TYPE_SINT32:
		return "integer", "int32", true
	case descriptorpb.FieldDescriptorProto_TYPE_SINT64:
		return "string", "int64", true
	default:
		return "", "", false
	}
}

// renderEnumerationsAsDefinition inserts enums into the definitions object.
func renderEnumerationsAsDefinition(enums enumMap, d openapiDefinitionsObject, reg *descriptor.Registry, customRefs refMap) {
	for _, enum := range enums {
		swgName, ok := fullyQualifiedNameToOpenAPIName(enum.FQEN(), reg)
		if !ok {
			panic(fmt.Sprintf("can't resolve OpenAPI name from FQEN %q", enum.FQEN()))
		}
		enumComments := protoComments(reg, enum.File, enum.Outers, "EnumType", int32(enum.Index))

		// it may be necessary to sort the result of the GetValue function.
		enumNames := listEnumNames(reg, enum)
		defaultValue := getEnumDefault(reg, enum)
		valueComments := enumValueProtoComments(reg, enum)
		if valueComments != "" {
			enumComments = strings.TrimLeft(enumComments+"\n\n "+valueComments, "\n")
		}
		enumSchemaObject := openapiSchemaObject{
			schemaCore: schemaCore{
				Type:    "string",
				Enum:    enumNames,
				Default: defaultValue,
			},
		}

		if reg.GetEnumsAsInts() {
			enumSchemaObject.Type = "integer"
			enumSchemaObject.Format = "int32"
			enumSchemaObject.Default = getEnumDefaultNumber(reg, enum)
			enumSchemaObject.Enum = listEnumNumbers(reg, enum)
		}
		opts, err := getEnumOpenAPIOption(reg, enum)
		if err != nil {
			panic(err)
		}
		if opts != nil {
			protoSchema := openapiSchemaFromProtoEnumSchema(opts, reg, customRefs, enum)
			// Warning: Make sure not to overwrite any fields already set on the schema type.
			// This is only a subset of the fields from JsonSchema since most of them only apply to arrays or objects not enums
			enumSchemaObject.ExternalDocs = protoSchema.ExternalDocs
			enumSchemaObject.ReadOnly = protoSchema.ReadOnly
			enumSchemaObject.extensions = protoSchema.extensions
			if protoSchema.Type != "" || protoSchema.Ref != "" {
				enumSchemaObject.schemaCore = protoSchema.schemaCore
			}
			if protoSchema.Title != "" {
				enumSchemaObject.Title = protoSchema.Title
			}
			if protoSchema.Description != "" {
				enumSchemaObject.Description = protoSchema.Description
			}
			if protoSchema.Example != nil {
				enumSchemaObject.Example = protoSchema.Example
			}
		}
		if err := updateOpenAPIDataFromComments(reg, &enumSchemaObject, enum, enumComments, false); err != nil {
			panic(err)
		}

		d[swgName] = enumSchemaObject
	}
}

// Take in a FQMN or FQEN and return a OpenAPI safe version of the FQMN and
// a boolean indicating if FQMN was properly resolved.
func fullyQualifiedNameToOpenAPIName(fqn string, reg *descriptor.Registry) (string, bool) {
	registriesSeenMutex.Lock()
	defer registriesSeenMutex.Unlock()
	if mapping, present := registriesSeen[reg]; present {
		ret, ok := mapping[fqn]
		return ret, ok
	}
	mapping := resolveFullyQualifiedNameToOpenAPINames(append(reg.GetAllFQMNs(), append(reg.GetAllFQENs(), reg.GetAllFQMethNs()...)...), reg.GetOpenAPINamingStrategy())
	registriesSeen[reg] = mapping
	ret, ok := mapping[fqn]
	return ret, ok
}

// Lookup message type by location.name and return an openapiv2-safe version
// of its FQMN.
func lookupMsgAndOpenAPIName(location, name string, reg *descriptor.Registry) (*descriptor.Message, string, error) {
	msg, err := reg.LookupMsg(location, name)
	if err != nil {
		return nil, "", err
	}
	swgName, ok := fullyQualifiedNameToOpenAPIName(msg.FQMN(), reg)
	if !ok {
		return nil, "", fmt.Errorf("can't map OpenAPI name from FQMN %q", msg.FQMN())
	}
	return msg, swgName, nil
}

// registriesSeen is used to memoise calls to resolveFullyQualifiedNameToOpenAPINames so
// we don't repeat it unnecessarily, since it can take some time.
var (
	registriesSeen      = map[*descriptor.Registry]map[string]string{}
	registriesSeenMutex sync.Mutex
)

// Take the names of every proto message and generate a unique reference for each, according to the given strategy.
func resolveFullyQualifiedNameToOpenAPINames(messages []string, namingStrategy string) map[string]string {
	strategyFn := LookupNamingStrategy(namingStrategy)
	if strategyFn == nil {
		return nil
	}
	return strategyFn(messages)
}

var canRegexp = regexp.MustCompile("{([a-zA-Z][a-zA-Z0-9_.-]*)([^}]*)}")

// templateToParts splits a URL template into path segments for use by `partsToOpenAPIPath` and `partsToRegexpMap`.
//
// Parameters:
//   - path:	The URL template as defined by https://github.com/googleapis/googleapis/blob/master/google/api/http.proto
//   - reg:	The descriptor registry used to read compiler flags
//   - fields:	The fields of the request message, only used when `useJSONNamesForFields` is true
//   - msgs:	The Messages of the service binding, only used when `useJSONNamesForFields` is true
//
// Returns:
//
//	The path segments of the URL template.
func templateToParts(path string, reg *descriptor.Registry, fields []*descriptor.Field, msgs []*descriptor.Message) []string {
	// It seems like the right thing to do here is to just use
	// strings.Split(path, "/") but that breaks badly when you hit a url like
	// /{my_field=prefix/*}/ and end up with 2 sections representing my_field.
	// Instead do the right thing and write a small pushdown (counter) automata
	// for it.
	var parts []string
	depth := 0
	buffer := ""
	for i, char := range path {
		switch char {
		case '{':
			// Push on the stack
			depth++
			buffer += string(char)
		case '}':
			if depth == 0 {
				panic("Encountered } without matching { before it.")
			}
			// Pop from the stack
			depth--
			if !reg.GetUseJSONNamesForFields() {
				buffer += string(char)
				continue
			}
			paramNameProto := strings.SplitN(buffer[1:], "=", 2)[0]
			paramNameCamelCase := lowerCamelCase(paramNameProto, fields, msgs)
			buffer = strings.Join([]string{"{", paramNameCamelCase, buffer[len(paramNameProto)+1:], "}"}, "")
		case '/':
			if depth == 0 {
				parts = append(parts, buffer)
				buffer = ""
				// Since the stack was empty when we hit the '/' we are done with this
				// section.
				continue
			}
			buffer += string(char)
		case ':':
			if depth == 0 {
				// Only treat this as a verb if we're at the end of the path or
				// if there are no more path segments (only more literals after the colon)
				remainingPath := path[i:]
				if !strings.Contains(remainingPath, "/") {
					parts = append(parts, buffer)
					verbSegment := remainingPath
					if reg.GetUseJSONNamesForFields() {
						verbSegment = processParametersInSegment(verbSegment, fields, msgs)
					}
					parts = append(parts, verbSegment)
					return parts
				}
			}
			buffer += string(char)
		default:
			buffer += string(char)
		}
	}

	// Now append the last element to parts
	parts = append(parts, buffer)

	return parts
}

// processParametersInSegment processes a path segment (like ":verb/{param}") to convert
// parameter names to camelCase while preserving the overall structure
func processParametersInSegment(segment string, fields []*descriptor.Field, msgs []*descriptor.Message) string {
	result := segment
	depth := 0
	var paramStart int
	for i, char := range segment {
		switch char {
		case '{':
			if depth == 0 {
				paramStart = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 {
				paramContent := segment[paramStart+1 : i]
				paramNameProto := strings.SplitN(paramContent, "=", 2)[0]
				paramNameCamelCase := lowerCamelCase(paramNameProto, fields, msgs)

				oldParam := "{" + paramContent + "}"
				newParam := "{" + paramNameCamelCase
				if strings.Contains(paramContent, "=") {
					newParam += paramContent[len(paramNameProto):]
				}
				newParam += "}"

				result = strings.Replace(result, oldParam, newParam, 1)
			}
		}
	}
	return result
}

// partsToOpenAPIPath converts each path part of the form /path/{string_value=strprefix/*} which is defined in
// https://github.com/googleapis/googleapis/blob/master/google/api/http.proto to the OpenAPI expected form /path/{string_value}.
// For example this would replace the path segment of "{foo=bar/*}" with "{foo}" or "prefix{bang=bash/**}" with "prefix{bang}".
// OpenAPI 2 only allows simple path parameters with the constraints on that parameter specified in the OpenAPI
// schema's "pattern" instead of in the path parameter itself.
func partsToOpenAPIPath(parts []string, overrides map[string]string) string {
	for index, part := range parts {
		part = canRegexp.ReplaceAllString(part, "{$1}")

		if override, ok := overrides[part]; ok {
			part = override
		}
		parts[index] = part
	}
	if last := len(parts) - 1; strings.HasPrefix(parts[last], ":") {
		// Last item is a verb (":" LITERAL).
		return strings.Join(parts[:last], "/") + parts[last]
	}
	return strings.Join(parts, "/")
}

// partsToRegexpMap returns a map of parameter name to ECMA 262 patterns
// which is what the "pattern" field on an OpenAPI parameter expects.
// See https://swagger.io/specification/v2/ (Parameter Object) and
// https://tools.ietf.org/html/draft-fge-json-schema-validation-00#section-5.2.3.
// The expression is generated based on expressions defined by https://github.com/googleapis/googleapis/blob/master/google/api/http.proto
// "Path Template Syntax" section which allow for a "param_name=foobar/*/bang/**" style expressions inside
// the path parameter placeholders that indicate constraints on the values of those parameters.
// This function will scan the split parts of a path template for parameters and
// outputs a map of the name of the parameter to a ECMA regular expression.  See the http.proto file for descriptions
// of the supported syntax. This function will ignore any path parameters that don't contain a "=" after the
// parameter name.  For supported parameters, we assume "*" represent all characters except "/" as it's
// intended to match a single path element and we assume "**" matches any character as it's intended to match multiple
// path elements.
// For example "{name=organizations/*/roles/*}" would produce the regular expression for the "name" parameter of
// "organizations/[^/]+/roles/[^/]+" or "{bar=bing/*/bang/**}" would produce the regular expression for the "bar"
// parameter of "bing/[^/]+/bang/.+".
//
// Note that OpenAPI does not actually support path parameters with "/", see https://github.com/OAI/OpenAPI-Specification/issues/892
func partsToRegexpMap(parts []string) map[string]string {
	regExps := make(map[string]string)
	for _, part := range parts {
		if strings.Contains(part, "/") {
			grpclog.Warningf("Path parameter %q contains '/', which is not supported in OpenAPI", part)
		}
		if submatch := canRegexp.FindStringSubmatch(part); len(submatch) > 2 {
			if strings.HasPrefix(submatch[2], "=") { // this part matches the standard and should be made into a regular expression
				// assume the string's characters other than "**" and "*" are literals (not necessarily a good assumption 100% of the times, but it will support most use cases)
				regex := submatch[2][1:]
				regex = strings.ReplaceAll(regex, "**", ".+")   // ** implies any character including "/"
				regex = strings.ReplaceAll(regex, "*", "[^/]+") // * implies any character except "/"
				regExps[submatch[1]] = regex
			}
		}
	}
	return regExps
}

func renderServiceTags(services []*descriptor.Service, reg *descriptor.Registry) []openapiTagObject {
	var tags []openapiTagObject
	for _, svc := range services {
		if !isVisible(getServiceVisibilityOption(svc), reg) {
			continue
		}
		tagName := svc.GetName()
		if pkg := svc.File.GetPackage(); pkg != "" && reg.IsIncludePackageInTags() {
			tagName = pkg + "." + tagName
		}

		tag := openapiTagObject{
			Name: tagName,
		}

		opts, err := getServiceOpenAPIOption(reg, svc)
		if err != nil {
			grpclog.Error(err)
			return nil
		}
		if opts != nil {
			tag.Description = opts.Description
			if reg.GetUseGoTemplate() {
				tag.Description = goTemplateComments(tag.Description, svc, reg)
			}
			if opts.ExternalDocs != nil {
				tag.ExternalDocs = &openapiExternalDocumentationObject{
					Description: opts.ExternalDocs.Description,
					URL:         opts.ExternalDocs.Url,
				}
				if reg.GetUseGoTemplate() {
					tag.ExternalDocs.Description = goTemplateComments(opts.ExternalDocs.Description, svc, reg)
				}
			}
			if opts.GetName() != "" {
				tag.Name = opts.GetName()
			}
		}
		tags = append(tags, tag)
	}
	return tags
}

// expandPathPatterns searches the URI parts for path parameters with pattern and when the pattern contains a sub-path,
// it expands the pattern into the URI parts and adds the new path parameters to the pathParams slice.
//
// Parameters:
//   - pathParts:	the URI parts parsed from the path template with `templateToParts` function
//   - pathParams: the path parameters of the service binding
//
// Returns:
//
//	The modified pathParts and pathParams slice.
func expandPathPatterns(pathParts []string, pathParams []descriptor.Parameter, reg *descriptor.Registry) ([]string, []descriptor.Parameter) {
	expandedPathParts := []string{}
	modifiedPathParams := pathParams
	for _, pathPart := range pathParts {
		if !strings.HasPrefix(pathPart, "{") || !strings.HasSuffix(pathPart, "}") {
			expandedPathParts = append(expandedPathParts, pathPart)
			continue
		}
		woBraces := pathPart[1 : len(pathPart)-1]
		paramPattern := strings.SplitN(woBraces, "=", 2)
		if len(paramPattern) != 2 {
			expandedPathParts = append(expandedPathParts, pathPart)
			continue
		}
		paramName := paramPattern[0]
		pattern := paramPattern[1]
		if pattern == "*" {
			expandedPathParts = append(expandedPathParts, pathPart)
			continue
		}
		pathParamIndex := slices.IndexFunc(modifiedPathParams, func(p descriptor.Parameter) bool {
			if !reg.GetUseJSONNamesForFields() {
				return p.FieldPath.String() == paramName
			}
			fieldPath := casing.JSONCamelCase(p.FieldPath.String())
			return fieldPath == paramName
		})
		if pathParamIndex == -1 {
			panic(fmt.Sprintf("Path parameter %q not found in path parameters", paramName))
		}
		pathParam := modifiedPathParams[pathParamIndex]
		patternParts := strings.Split(pattern, "/")
		for _, patternPart := range patternParts {
			if patternPart != "*" {
				expandedPathParts = append(expandedPathParts, patternPart)
				continue
			}
			lastPart := expandedPathParts[len(expandedPathParts)-1]
			paramName := strings.TrimSuffix(lastPart, "s")
			if reg.GetUseJSONNamesForFields() {
				paramName = casing.JSONCamelCase(paramName)
			}
			expandedPathParts = append(expandedPathParts, "{"+paramName+"}")
			newParam := descriptor.Parameter{
				Target: &descriptor.Field{
					FieldDescriptorProto: &descriptorpb.FieldDescriptorProto{
						Name: proto.String(paramName),
						Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					},
					Message:           pathParam.Target.Message,
					FieldMessage:      pathParam.Target.FieldMessage,
					ForcePrefixedName: pathParam.Target.ForcePrefixedName,
				},
				FieldPath: []descriptor.FieldPathComponent{{
					Name:   paramName,
					Target: nil,
				}},
				Method: nil,
			}
			modifiedPathParams = append(modifiedPathParams, newParam)
			if pathParamIndex != -1 {
				// the new parameter from the pattern replaces the old path parameter
				modifiedPathParams = append(modifiedPathParams[:pathParamIndex], modifiedPathParams[pathParamIndex+1:]...)
				pathParamIndex = -1
			}
		}
	}
	return expandedPathParts, modifiedPathParams
}

func renderServices(services []*descriptor.Service, paths *openapiPathsObject, reg *descriptor.Registry, requestResponseRefs, customRefs refMap, msgs []*descriptor.Message, defs openapiDefinitionsObject) error {
	// Correctness of svcIdx and methIdx depends on 'services' containing the services in the same order as the 'file.Service' array.
	svcBaseIdx := 0
	var lastFile *descriptor.File = nil
	for svcIdx, svc := range services {
		if svc.File != lastFile {
			lastFile = svc.File
			svcBaseIdx = svcIdx
		}

		if !isVisible(getServiceVisibilityOption(svc), reg) {
			continue
		}

		for methIdx, meth := range svc.Methods {
			if !isVisible(getMethodVisibilityOption(meth), reg) {
				continue
			}

			deprecated := reg.GetEnableRpcDeprecation() && meth.GetOptions().GetDeprecated()

			for bIdx, b := range meth.Bindings {
				operationFunc := operationForMethod(b.HTTPMethod)
				// Iterate over all the OpenAPI parameters
				parameters := openapiParametersObject{}
				// split the path template into its parts
				parts := templateToParts(b.PathTmpl.Template, reg, meth.RequestType.Fields, msgs)
				pathParams := b.PathParams
				if reg.GetExpandSlashedPathPatterns() {
					parts, pathParams = expandPathPatterns(parts, pathParams, reg)
				}
				// extract any constraints specified in the path placeholders into ECMA regular expressions
				pathParamRegexpMap := partsToRegexpMap(parts)
				// Keep track of path parameter overrides
				pathParamNames := make(map[string]string)
				for _, parameter := range pathParams {

					var paramType, paramFormat, desc, collectionFormat string
					var defaultValue interface{}
					var enumNames interface{}
					var items *openapiItemsObject
					var minItems *int
					var extensions []extension
					switch pt := parameter.Target.GetType(); pt {
					case descriptorpb.FieldDescriptorProto_TYPE_GROUP, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
						if descriptor.IsWellKnownType(parameter.Target.GetTypeName()) {
							if parameter.IsRepeated() {
								return errors.New("only primitive and enum types are allowed in repeated path parameters")
							}
							schema := schemaOfField(parameter.Target, reg, customRefs)
							paramType = schema.Type
							paramFormat = schema.Format
							desc = schema.Description
							defaultValue = schema.Default
							extensions = schema.extensions
						} else {
							return errors.New("only primitive and well-known types are allowed in path parameters")
						}
					case descriptorpb.FieldDescriptorProto_TYPE_ENUM:
						enum, err := reg.LookupEnum("", parameter.Target.GetTypeName())
						if err != nil {
							return err
						}
						paramType = "string"
						paramFormat = ""
						enumNames = listEnumNames(reg, enum)
						if reg.GetEnumsAsInts() {
							paramType = "integer"
							paramFormat = ""
							enumNames = listEnumNumbers(reg, enum)
						}

						schema := schemaOfField(parameter.Target, reg, customRefs)
						desc = schema.Description
						defaultValue = schema.Default
						extensions = schema.extensions
					default:
						var ok bool
						paramType, paramFormat, ok = primitiveSchema(pt)
						if !ok {
							return fmt.Errorf("unknown field type %v", pt)
						}

						schema := schemaOfField(parameter.Target, reg, customRefs)
						desc = schema.Description
						defaultValue = schema.Default
						extensions = schema.extensions
						// If there is no mandatory format based on the field,
						// allow it to be overridden by the user
						if paramFormat == "" {
							paramFormat = schema.Format
						}
					}

					if parameter.IsRepeated() {
						core := schemaCore{Type: paramType, Format: paramFormat}
						if parameter.IsEnum() {
							core.Enum = enumNames
							enumNames = nil
						}
						items = (*openapiItemsObject)(&openapiSchemaObject{schemaCore: core})
						paramType = "array"
						paramFormat = ""
						collectionFormat = reg.GetRepeatedPathParamSeparatorName()
						minItems = new(int)
						*minItems = 1
					}

					if desc == "" {
						desc = fieldProtoComments(reg, parameter.Target.Message, parameter.Target)
					}
					parameterString := parameter.String()
					if reg.GetUseJSONNamesForFields() {
						parameterString = lowerCamelCase(parameterString, meth.RequestType.Fields, msgs)
					}
					var pattern string
					if regExp, ok := pathParamRegexpMap[parameterString]; ok {
						pattern = regExp
					}
					fc := getFieldConfiguration(reg, parameter.Target)
					if fc != nil {
						pathParamName := fc.GetPathParamName()
						if pathParamName != "" && pathParamName != parameterString {
							pathParamNames["{"+parameterString+"}"] = "{" + pathParamName + "}"
							parameterString, _, _ = strings.Cut(pathParamName, "=")
						}
					}

					// verify if the parameter is deprecated, either via proto or annotation
					protoDeprecated := parameter.Target.GetOptions().GetDeprecated() && reg.GetEnableFieldDeprecation()
					annotationDeprecated := fc.GetDeprecated()
					deprecated := protoDeprecated || annotationDeprecated

					parameters = append(parameters, openapiParameterObject{
						Name:        parameterString,
						Description: desc,
						In:          "path",
						Required:    true,
						Deprecated:  deprecated,
						Default:     defaultValue,
						// Parameters in gRPC-Gateway can only be strings?
						Type:             paramType,
						Format:           paramFormat,
						Enum:             enumNames,
						Items:            items,
						CollectionFormat: collectionFormat,
						MinItems:         minItems,
						Pattern:          pattern,
						extensions:       extensions,
					})
				}
				// Now check if there is a body parameter
				if b.Body != nil {
					// Recursively render fields as definitions as long as they contain path parameters.
					// Special case for top level body if we don't have a body field.
					var schema openapiSchemaObject
					desc := ""
					var bodyFieldName string
					schema = openapiSchemaObject{
						schemaCore: schemaCore{},
					}
					if len(b.Body.FieldPath) == 0 {
						// No field for body, use type.
						bodyFieldName = "body"
						wknSchemaCore, isWkn := wktSchemas[meth.RequestType.FQMN()]
						if isWkn {
							schema.schemaCore = wknSchemaCore
							// Special workaround for Empty: it's well-known type but wknSchemas only returns schema.schemaCore; but we need to set schema.Properties which is a level higher.
							if meth.RequestType.FQMN() == ".google.protobuf.Empty" {
								schema.Properties = &openapiSchemaObjectProperties{}
							}
						} else {
							messageSchema, err := renderMessageAsDefinition(meth.RequestType, reg, customRefs, b.PathParams)
							if err != nil {
								return err
							}
							if len(b.PathParams) == 0 {
								if err := schema.setRefFromFQN(meth.RequestType.FQMN(), reg); err != nil {
									return err
								}
								desc = messageSchema.Description
							} else {
								if meth.Name != nil {
									methFQN, ok := fullyQualifiedNameToOpenAPIName(meth.FQMN(), reg)
									if !ok {
										panic(fmt.Errorf("failed to resolve method FQN: '%s'", meth.FQMN()))
									}
									defName := methFQN + "Body"
									schema.Ref = fmt.Sprintf("#/definitions/%s", defName)
									defs[defName] = messageSchema
								} else {
									schema = messageSchema
									if schema.Properties == nil || len(*schema.Properties) == 0 {
										grpclog.Warningf("created a body with 0 properties in the message, this might be unintended: %s", *meth.RequestType)
									}
								}
							}
						}
					} else {
						// Body field path is limited to one path component. From google.api.HttpRule.body:
						// "NOTE: the referred field must be present at the top-level of the request message type."
						// Ref: https://github.com/googleapis/googleapis/blob/b3397f5febbf21dfc69b875ddabaf76bee765058/google/api/http.proto#L350-L352
						if len(b.Body.FieldPath) > 1 {
							return fmt.Errorf("body of request %q is not a top level field: '%v'", meth.Service.GetName(), b.Body.FieldPath)
						}
						bodyField := b.Body.FieldPath[0]
						if reg.GetUseJSONNamesForFields() {
							bodyFieldName = lowerCamelCase(bodyField.Name, meth.RequestType.Fields, msgs)
						} else {
							bodyFieldName = bodyField.Name
						}
						// Align pathParams with body field path.
						pathParams := subPathParams(bodyField.Name, b.PathParams)

						if len(pathParams) == 0 {
							// When there are no path parameters, we only need the base schema of the field.
							// https://github.com/grpc-ecosystem/grpc-gateway/issues/3058
							schema = schemaOfFieldBase(bodyField.Target, reg, customRefs)
						} else {
							var err error
							schema, err = renderFieldAsDefinition(bodyField.Target, reg, customRefs, pathParams)
							if err != nil {
								return err
							}
							// renderFieldAsDefinition may add the body field name to the schema's required array
							// via updateSwaggerObjectFromFieldBehavior. However, for body parameters, the schema
							// represents the field's type, not the containing message. The body field name should
							// only be in the schema's required array if it's actually a property of the schema.
							// Remove the body field name from required if it's not a property to avoid invalid entries.
							if schema.Required != nil && schema.Properties != nil {
								// Build a set of property names
								propertyNames := make(map[string]bool)
								for _, prop := range *schema.Properties {
									propertyNames[prop.Key] = true
								}
								// Filter required array: keep field names that are either:
								// 1. Not the body field name, OR
								// 2. The body field name AND it's actually a property
								filteredRequired := make([]string, 0, len(schema.Required))
								seenBodyFieldName := false
								for _, req := range schema.Required {
									if req == bodyFieldName {
										if propertyNames[req] {
											// It's a property, keep it (but only once)
											if !seenBodyFieldName {
												filteredRequired = append(filteredRequired, req)
												seenBodyFieldName = true
											}
										}
										// else: It's not a property, skip it
									} else {
										// Not the body field name, keep it
										filteredRequired = append(filteredRequired, req)
									}
								}
								schema.Required = filteredRequired
							}
						}
						if schema.Title != "" {
							desc = mergeDescription(schema)
						} else {
							desc = fieldProtoComments(reg, bodyField.Target.Message, bodyField.Target)
						}
					}

					if meth.GetClientStreaming() {
						desc += " (streaming inputs)"
					}
					parameters = append(parameters, openapiParameterObject{
						Name:        bodyFieldName,
						Description: desc,
						In:          "body",
						Required:    true,
						Schema:      &schema,
					})
				}

				// add the parameters to the query string
				queryParams, err := messageToQueryParameters(meth.RequestType, reg, b.PathParams, b.Body, b.HTTPMethod)
				if err != nil {
					return err
				}
				parameters = append(parameters, queryParams...)

				path := partsToOpenAPIPath(parts, pathParamNames)

				pathItemObject, ok := getPathItemObject(*paths, path)

				if !ok {
					pathItemObject = openapiPathItemObject{}
				} else {
					// handle case where we have an existing mapping for the same path and method
					existingOperationObject := operationFunc(&pathItemObject)
					if existingOperationObject != nil {
						var firstPathParameter *openapiParameterObject
						var firstParamIndex int
						for index, param := range parameters {
							param := param
							if param.In == "path" {
								firstPathParameter = &param
								firstParamIndex = index
								break
							}
						}
						if firstPathParameter == nil {
							// Without a path parameter, there is nothing to vary to support multiple mappings of the same path/method.
							// Previously this did not log an error and only overwrote the mapping, we now log the error but
							// still overwrite the mapping
							grpclog.Errorf("Duplicate mapping for path %s %s", b.HTTPMethod, path)
						} else {
							newPathCount := 0
							var newPath string
							var newPathElement string
							// Iterate until there is not an existing operation that matches the same escaped path.
							// Most of the time this will only be a single iteration, but a large API could technically have
							// a pretty large amount of these if it used similar patterns for all its functions.
							for existingOperationObject != nil {
								newPathCount += 1
								newPathElement = firstPathParameter.Name + pathParamUniqueSuffixDeliminator + strconv.Itoa(newPathCount)
								newPath = strings.ReplaceAll(path, "{"+firstPathParameter.Name+"}", "{"+newPathElement+"}")

								if newPathItemObject, ok := getPathItemObject(*paths, newPath); ok {
									existingOperationObject = operationFunc(&newPathItemObject)
								} else {
									existingOperationObject = nil
								}
							}
							// update the pathItemObject we are adding to with the new path
							pathItemObject, _ = getPathItemObject(*paths, newPath)
							firstPathParameter.Name = newPathElement
							path = newPath
							parameters[firstParamIndex] = *firstPathParameter
						}
					}
				}

				methProtoPath := protoPathIndex(reflect.TypeOf((*descriptorpb.ServiceDescriptorProto)(nil)), "Method")
				desc := "A successful response."
				var responseSchema openapiSchemaObject

				if b.ResponseBody == nil || len(b.ResponseBody.FieldPath) == 0 {
					responseSchema = openapiSchemaObject{
						schemaCore: schemaCore{},
					}

					// Don't link to a full definition for
					// empty; it's overly verbose.
					// schema.Properties{} renders it as
					// well, without a definition
					wknSchemaCore, isWkn := wktSchemas[meth.ResponseType.FQMN()]
					if !isWkn {
						if err := responseSchema.setRefFromFQN(meth.ResponseType.FQMN(), reg); err != nil {
							return err
						}
					} else {
						responseSchema.schemaCore = wknSchemaCore

						// Special workaround for Empty: it's well-known type but wknSchemas only returns schema.schemaCore; but we need to set schema.Properties which is a level higher.
						if meth.ResponseType.FQMN() == ".google.protobuf.Empty" {
							responseSchema.Properties = &openapiSchemaObjectProperties{}
						}
					}
				} else {
					// This is resolving the value of response_body in the google.api.HttpRule
					lastField := b.ResponseBody.FieldPath[len(b.ResponseBody.FieldPath)-1]
					responseSchema = schemaOfField(lastField.Target, reg, customRefs)
					if responseSchema.Description != "" {
						desc = responseSchema.Description
					} else {
						desc = fieldProtoComments(reg, lastField.Target.Message, lastField.Target)
					}
				}
				if meth.GetServerStreaming() {
					desc += "(streaming responses)"
					responseSchema.Type = "object"
					swgRef, _ := fullyQualifiedNameToOpenAPIName(meth.ResponseType.FQMN(), reg)
					responseSchema.Title = "Stream result of " + swgRef

					props := openapiSchemaObjectProperties{
						keyVal{
							Key: "result",
							Value: openapiSchemaObject{
								schemaCore: schemaCore{
									Ref: responseSchema.Ref,
								},
							},
						},
					}
					if !reg.GetDisableDefaultErrors() {
						statusDef, hasStatus := fullyQualifiedNameToOpenAPIName(".google.rpc.Status", reg)
						if hasStatus {
							props = append(props, keyVal{
								Key: "error",
								Value: openapiSchemaObject{
									schemaCore: schemaCore{
										Ref: fmt.Sprintf("#/definitions/%s", statusDef),
									},
								},
							})
						}
					}

					// Special case HttpBody responses, they will be unformatted bytes
					if meth.ResponseType.FQMN() == ".google.api.HttpBody" {
						responseSchema.Type = "string"
						responseSchema.Format = "binary"
						responseSchema.Title = "Free form byte stream"
						// The error response is still JSON, but technically the full response
						// is still unformatted, so don't include the error response structure.
						props = nil
					}

					responseSchema.Properties = &props
					responseSchema.Ref = ""
				}

				operationObject := &openapiOperationObject{
					Parameters: parameters,
					Responses:  openapiResponsesObject{},
					Deprecated: deprecated,
				}

				if !reg.GetDisableDefaultResponses() {
					operationObject.Responses["200"] = openapiResponseObject{
						Description: desc,
						Schema:      responseSchema,
						Headers:     openapiHeadersObject{},
					}
				}

				if !reg.GetDisableServiceTags() {
					tag := svc.GetName()
					if pkg := svc.File.GetPackage(); pkg != "" && reg.IsIncludePackageInTags() {
						tag = pkg + "." + tag
					}
					operationObject.Tags = []string{tag}
				}

				if !reg.GetDisableDefaultErrors() {
					errDef, hasErrDef := fullyQualifiedNameToOpenAPIName(".google.rpc.Status", reg)
					if hasErrDef {
						// https://github.com/OAI/OpenAPI-Specification/blob/3.0.0/versions/2.0.md#responses-object
						operationObject.Responses["default"] = openapiResponseObject{
							Description: "An unexpected error response.",
							Schema: openapiSchemaObject{
								schemaCore: schemaCore{
									Ref: fmt.Sprintf("#/definitions/%s", errDef),
								},
							},
						}
					}
				}
				operationObject.OperationID = fmt.Sprintf("%s_%s", svc.GetName(), meth.GetName())
				if reg.GetSimpleOperationIDs() {
					operationObject.OperationID = meth.GetName()
				}
				if bIdx != 0 {
					// OperationID must be unique in an OpenAPI v2 definition.
					operationObject.OperationID += strconv.Itoa(bIdx + 1)
				}

				// Fill reference map with referenced request messages
				for _, param := range operationObject.Parameters {
					if param.Schema != nil && param.Schema.Ref != "" {
						requestResponseRefs[param.Schema.Ref] = struct{}{}
					}
				}

				methComments := protoComments(reg, svc.File, nil, "Service", int32(svcIdx-svcBaseIdx), methProtoPath, int32(methIdx))
				if err := updateOpenAPIDataFromComments(reg, operationObject, meth, methComments, false); err != nil {
					panic(err)
				}

				svcOpts, err := getServiceOpenAPIOption(reg, svc)
				if err != nil {
					grpclog.Error(err)
					return err
				}

				// Set Tag with the user-defined service name
				if svcOpts.GetName() != "" {
					operationObject.Tags = []string{svcOpts.GetName()}
				}

				opts, err := getMethodOpenAPIOption(reg, meth)
				if opts != nil {
					if err != nil {
						panic(err)
					}
					operationObject.ExternalDocs = protoExternalDocumentationToOpenAPIExternalDocumentation(opts.ExternalDocs, reg, meth)

					if opts.Deprecated {
						operationObject.Deprecated = true
					}

					if opts.Summary != "" {
						operationObject.Summary = opts.Summary
					}
					if opts.Description != "" {
						operationObject.Description = opts.Description
					}
					if len(opts.Tags) > 0 {
						operationObject.Tags = make([]string, len(opts.Tags))
						copy(operationObject.Tags, opts.Tags)
					}
					if opts.OperationId != "" {
						operationObject.OperationID = opts.OperationId
					}
					if opts.Security != nil {
						newSecurity := []openapiSecurityRequirementObject{}
						if operationObject.Security != nil {
							newSecurity = *operationObject.Security
						}
						for _, secReq := range opts.Security {
							newSecReq := openapiSecurityRequirementObject{}
							for secReqKey, secReqValue := range secReq.SecurityRequirement {
								if secReqValue == nil {
									continue
								}

								newSecReqValue := make([]string, len(secReqValue.Scope))
								copy(newSecReqValue, secReqValue.Scope)
								newSecReq[secReqKey] = newSecReqValue
							}

							if len(newSecReq) > 0 {
								newSecurity = append(newSecurity, newSecReq)
							}
						}
						operationObject.Security = &newSecurity
					}
					if opts.Responses != nil {
						for name, resp := range opts.Responses {
							// Merge response data into default response if available.
							respObj := operationObject.Responses[name]
							if resp.Description != "" {
								respObj.Description = resp.Description
							}
							if resp.Schema != nil {
								respObj.Schema = openapiSchemaFromProtoSchema(resp.Schema, reg, customRefs, meth)
							}
							if resp.Examples != nil {
								respObj.Examples = openapiExamplesFromProtoExamples(resp.Examples)
							}
							if resp.Headers != nil {
								hdrs, err := processHeaders(resp.Headers)
								if err != nil {
									return err
								}
								respObj.Headers = hdrs
							}
							if resp.Extensions != nil {
								exts, err := processExtensions(resp.Extensions)
								if err != nil {
									return err
								}
								respObj.extensions = exts
							}
							operationObject.Responses[name] = respObj
						}
					}

					if opts.Extensions != nil {
						exts, err := processExtensions(opts.Extensions)
						if err != nil {
							return err
						}
						operationObject.extensions = exts
					}

					if len(opts.Consumes) > 0 {
						operationObject.Consumes = make([]string, len(opts.Consumes))
						copy(operationObject.Consumes, opts.Consumes)
					}

					if len(opts.Produces) > 0 {
						operationObject.Produces = make([]string, len(opts.Produces))
						copy(operationObject.Produces, opts.Produces)
					}

					if params := opts.Parameters; params != nil && len(params.Headers) > 0 {
						for _, header := range params.Headers {
							param := openapiParameterObject{
								In:          "header",
								Name:        header.Name,
								Description: header.Description,
								Required:    header.Required,
								Format:      header.Format,
							}

							switch header.Type {
							case openapi_options.HeaderParameter_STRING:
								param.Type = "string"
							case openapi_options.HeaderParameter_NUMBER:
								param.Type = "number"
							case openapi_options.HeaderParameter_INTEGER:
								param.Type = "integer"
							case openapi_options.HeaderParameter_BOOLEAN:
								param.Type = "boolean"
							default:
								return fmt.Errorf("invalid header parameter type: %+v", header.Type)
							}

							operationObject.Parameters = append(operationObject.Parameters, param)
						}
					}

					// TODO(ivucica): add remaining fields of operation object
				}

				switch b.HTTPMethod {
				case "DELETE":
					pathItemObject.Delete = operationObject
				case "GET":
					pathItemObject.Get = operationObject
				case "POST":
					pathItemObject.Post = operationObject
				case "PUT":
					pathItemObject.Put = operationObject
				case "PATCH":
					pathItemObject.Patch = operationObject
				case "HEAD":
					pathItemObject.Head = operationObject
				case "OPTIONS":
					pathItemObject.Options = operationObject
				}

				updatePaths(paths, path, pathItemObject)
			}
		}
	}

	// Success! return nil on the error object
	return nil
}

// Returns the openapiPathItemObject associated with a path. If path is not present, returns
// empty openapiPathItemObject and false.
func getPathItemObject(paths openapiPathsObject, path string) (openapiPathItemObject, bool) {
	for _, pathData := range paths {
		if pathData.Path == path {
			return pathData.PathItemObject, true
		}
	}

	return openapiPathItemObject{}, false
}

// If a path already exists in openapiPathsObject, updates that path's openapiPathItemObject. If not,
// appends a new path and openapiPathItemObject to the openapiPathsObject.
func updatePaths(paths *openapiPathsObject, path string, pathItemObject openapiPathItemObject) {
	for i, p := range *paths {
		if p.Path == path {
			(*paths)[i].PathItemObject = pathItemObject
			return
		}
	}
	*paths = append(*paths, pathData{
		Path:           path,
		PathItemObject: pathItemObject,
	})
}

func mergeDescription(schema openapiSchemaObject) string {
	desc := schema.Description
	if schema.Title != "" { // join title because title of parameter object will be ignored
		desc = strings.TrimSpace(schema.Title + paragraphDeliminator + schema.Description)
	}
	return desc
}

func operationForMethod(httpMethod string) func(*openapiPathItemObject) *openapiOperationObject {
	switch httpMethod {
	case "GET":
		return func(obj *openapiPathItemObject) *openapiOperationObject { return obj.Get }
	case "POST":
		return func(obj *openapiPathItemObject) *openapiOperationObject { return obj.Post }
	case "PUT":
		return func(obj *openapiPathItemObject) *openapiOperationObject { return obj.Put }
	case "DELETE":
		return func(obj *openapiPathItemObject) *openapiOperationObject { return obj.Delete }
	case "PATCH":
		return func(obj *openapiPathItemObject) *openapiOperationObject { return obj.Patch }
	case "HEAD":
		return func(obj *openapiPathItemObject) *openapiOperationObject { return obj.Head }
	case "OPTIONS":
		return func(obj *openapiPathItemObject) *openapiOperationObject { return obj.Options }
	default:
		return func(obj *openapiPathItemObject) *openapiOperationObject { return nil }
	}
}

// This function is called with a param which contains the entire definition of a method.
func applyTemplate(p param) (*openapiSwaggerObject, error) {
	// Create the basic template object. This is the object that everything is
	// defined off of.
	s := openapiSwaggerObject{
		// OpenAPI 2.0 is the version of this document
		Swagger:     "2.0",
		Consumes:    []string{"application/json"},
		Produces:    []string{"application/json"},
		Paths:       openapiPathsObject{},
		Definitions: make(openapiDefinitionsObject),
		Info: openapiInfoObject{
			Title:   *p.File.Name,
			Version: "version not set",
		},
	}

	// IMPORTANT: Initialize the naming cache BEFORE any code that uses fullyQualifiedNameToOpenAPIName.
	// This ensures consistent naming between renderServices (which generates $refs) and
	// renderMessagesAsDefinition (which generates definitions).
	//
	// Pre-scan to collect referenced names WITHOUT using the naming cache.
	// This allows us to build the cache with the correct filtered names upfront.
	referencedNames := collectReferencedNamesForCache(p.Services, p.Messages, p.reg)

	// Get all names from the registry
	allFQMNs := p.reg.GetAllFQMNs()
	allFQENs := p.reg.GetAllFQENs()
	allFQMethNs := p.reg.GetAllFQMethNs()
	allNames := append(append(allFQMNs, allFQENs...), allFQMethNs...)

	// Filter: EXCLUDE names that are from a DIFFERENT package AND are NOT referenced
	// This way we keep all names from the current package, and all referenced names from other packages
	currentPackage := p.File.GetPackage()
	filteredNames := make([]string, 0, len(allNames))
	for _, name := range allNames {
		trimmedName := strings.TrimPrefix(name, ".")
		if trimmedName == "" {
			continue
		}
		// Include if: (1) from current package, OR (2) actually referenced, OR (3) from google.*/grpc.* packages
		isCurrentPackage := strings.HasPrefix(trimmedName, currentPackage+".")
		isGoogle := strings.HasPrefix(trimmedName, "google.")
		isGRPC := strings.HasPrefix(trimmedName, "grpc.")
		if isCurrentPackage || referencedNames[name] || isGoogle || isGRPC {
			filteredNames = append(filteredNames, name)
		}
	}

	// Initialize the naming cache BEFORE renderServices so all lookups use consistent naming
	registriesSeenMutex.Lock()
	resolvedNames := resolveFullyQualifiedNameToOpenAPINames(filteredNames, p.reg.GetOpenAPINamingStrategy())
	registriesSeen[p.reg] = resolvedNames
	registriesSeenMutex.Unlock()

	// Loops through all the services and their exposed GET/POST/PUT/DELETE definitions
	// and create entries for all of them.
	// Also adds custom user specified references to second map.
	// NOTE: This now uses the naming cache initialized above.
	requestResponseRefs, customRefs := refMap{}, refMap{}
	if err := renderServices(p.Services, &s.Paths, p.reg, requestResponseRefs, customRefs, p.Messages, s.Definitions); err != nil {
		panic(err)
	}

	messages := messageMap{}
	streamingMessages := messageMap{}
	enums := enumMap{}

	if !p.reg.GetDisableDefaultErrors() {
		// Add the error type to the message map
		runtimeError, swgRef, err := lookupMsgAndOpenAPIName("google.rpc", "Status", p.reg)
		if err == nil {
			messages[swgRef] = runtimeError
		} else {
			// just in case there is an error looking up runtimeError
			grpclog.Error(err)
		}
	}

	// Find all the service's messages and enumerations that are defined (recursively)
	// and write request, response and other custom (but referenced) types out as definition objects.
	// NOTE: This uses the same naming cache that was used by renderServices above.
	findServicesMessagesAndEnumerations(p.Services, p.reg, messages, streamingMessages, enums, requestResponseRefs)

	if err := renderMessagesAsDefinition(messages, s.Definitions, p.reg, customRefs, nil); err != nil {
		return nil, err
	}
	renderEnumerationsAsDefinition(enums, s.Definitions, p.reg, requestResponseRefs)

	// File itself might have some comments and metadata.
	packageProtoPath := protoPathIndex(reflect.TypeOf((*descriptorpb.FileDescriptorProto)(nil)), "Package")
	packageComments := protoComments(p.reg, p.File, nil, "Package", packageProtoPath)
	if err := updateOpenAPIDataFromComments(p.reg, &s, p, packageComments, true); err != nil {
		return nil, err
	}

	// There may be additional options in the OpenAPI option in the proto.
	spb, err := getFileOpenAPIOption(p.reg, p.File)
	if err != nil {
		return nil, err
	}
	if spb != nil {
		if spb.Swagger != "" {
			s.Swagger = spb.Swagger
		}
		if spb.Info != nil {
			if spb.Info.Title != "" {
				s.Info.Title = spb.Info.Title
			}
			if spb.Info.Description != "" {
				s.Info.Description = spb.Info.Description
			}
			if spb.Info.TermsOfService != "" {
				s.Info.TermsOfService = spb.Info.TermsOfService
			}
			if spb.Info.Version != "" {
				s.Info.Version = spb.Info.Version
			}
			if spb.Info.Contact != nil {
				if s.Info.Contact == nil {
					s.Info.Contact = &openapiContactObject{}
				}
				if spb.Info.Contact.Name != "" {
					s.Info.Contact.Name = spb.Info.Contact.Name
				}
				if spb.Info.Contact.Url != "" {
					s.Info.Contact.URL = spb.Info.Contact.Url
				}
				if spb.Info.Contact.Email != "" {
					s.Info.Contact.Email = spb.Info.Contact.Email
				}
			}
			if spb.Info.License != nil {
				if s.Info.License == nil {
					s.Info.License = &openapiLicenseObject{}
				}
				if spb.Info.License.Name != "" {
					s.Info.License.Name = spb.Info.License.Name
				}
				if spb.Info.License.Url != "" {
					s.Info.License.URL = spb.Info.License.Url
				}
			}
			if spb.Info.Extensions != nil {
				exts, err := processExtensions(spb.Info.Extensions)
				if err != nil {
					return nil, err
				}
				s.Info.extensions = exts
			}
		}
		if spb.Host != "" {
			s.Host = spb.Host
		}
		if spb.BasePath != "" {
			s.BasePath = spb.BasePath
		}
		if len(spb.Schemes) > 0 {
			s.Schemes = make([]string, len(spb.Schemes))
			for i, scheme := range spb.Schemes {
				s.Schemes[i] = strings.ToLower(scheme.String())
			}
		}
		if len(spb.Consumes) > 0 {
			s.Consumes = make([]string, len(spb.Consumes))
			copy(s.Consumes, spb.Consumes)
		}
		if len(spb.Produces) > 0 {
			s.Produces = make([]string, len(spb.Produces))
			copy(s.Produces, spb.Produces)
		}
		if spb.SecurityDefinitions != nil && spb.SecurityDefinitions.Security != nil {
			if s.SecurityDefinitions == nil {
				s.SecurityDefinitions = openapiSecurityDefinitionsObject{}
			}
			for secDefKey, secDefValue := range spb.SecurityDefinitions.Security {
				var newSecDefValue openapiSecuritySchemeObject
				if oldSecDefValue, ok := s.SecurityDefinitions[secDefKey]; !ok {
					newSecDefValue = openapiSecuritySchemeObject{}
				} else {
					newSecDefValue = oldSecDefValue
				}
				if secDefValue.Type != openapi_options.SecurityScheme_TYPE_INVALID {
					switch secDefValue.Type {
					case openapi_options.SecurityScheme_TYPE_BASIC:
						newSecDefValue.Type = "basic"
					case openapi_options.SecurityScheme_TYPE_API_KEY:
						newSecDefValue.Type = "apiKey"
					case openapi_options.SecurityScheme_TYPE_OAUTH2:
						newSecDefValue.Type = "oauth2"
					}
				}
				if secDefValue.Description != "" {
					newSecDefValue.Description = secDefValue.Description
				}
				if secDefValue.Name != "" {
					newSecDefValue.Name = secDefValue.Name
				}
				if secDefValue.In != openapi_options.SecurityScheme_IN_INVALID {
					switch secDefValue.In {
					case openapi_options.SecurityScheme_IN_QUERY:
						newSecDefValue.In = "query"
					case openapi_options.SecurityScheme_IN_HEADER:
						newSecDefValue.In = "header"
					}
				}
				if secDefValue.Flow != openapi_options.SecurityScheme_FLOW_INVALID {
					switch secDefValue.Flow {
					case openapi_options.SecurityScheme_FLOW_IMPLICIT:
						newSecDefValue.Flow = "implicit"
					case openapi_options.SecurityScheme_FLOW_PASSWORD:
						newSecDefValue.Flow = "password"
					case openapi_options.SecurityScheme_FLOW_APPLICATION:
						newSecDefValue.Flow = "application"
					case openapi_options.SecurityScheme_FLOW_ACCESS_CODE:
						newSecDefValue.Flow = "accessCode"
					}
				}
				if secDefValue.AuthorizationUrl != "" {
					newSecDefValue.AuthorizationURL = secDefValue.AuthorizationUrl
				}
				if secDefValue.TokenUrl != "" {
					newSecDefValue.TokenURL = secDefValue.TokenUrl
				}
				if secDefValue.Scopes != nil {
					if newSecDefValue.Scopes == nil {
						newSecDefValue.Scopes = openapiScopesObject{}
					}
					for scopeKey, scopeDesc := range secDefValue.Scopes.Scope {
						newSecDefValue.Scopes[scopeKey] = scopeDesc
					}
				}
				if secDefValue.Extensions != nil {
					exts, err := processExtensions(secDefValue.Extensions)
					if err != nil {
						return nil, err
					}
					newSecDefValue.extensions = exts
				}
				s.SecurityDefinitions[secDefKey] = newSecDefValue
			}
		}
		if spb.Security != nil {
			var newSecurity []openapiSecurityRequirementObject
			if s.Security != nil {
				newSecurity = s.Security
			}
			for _, secReq := range spb.Security {
				newSecReq := openapiSecurityRequirementObject{}
				for secReqKey, secReqValue := range secReq.SecurityRequirement {
					if secReqValue == nil {
						return nil, fmt.Errorf("malformed security requirement spec for key %q; value is required", secReqKey)
					}
					newSecReqValue := make([]string, len(secReqValue.Scope))
					copy(newSecReqValue, secReqValue.Scope)
					newSecReq[secReqKey] = newSecReqValue
				}
				newSecurity = append(newSecurity, newSecReq)
			}
			s.Security = newSecurity
		}
		s.ExternalDocs = protoExternalDocumentationToOpenAPIExternalDocumentation(spb.ExternalDocs, p.reg, spb)
		// Populate all Paths with Responses set at top level,
		// preferring Responses already set over those at the top level.
		if spb.Responses != nil {
			for _, verbs := range s.Paths {
				var maps []openapiResponsesObject
				if verbs.PathItemObject.Delete != nil {
					maps = append(maps, verbs.PathItemObject.Delete.Responses)
				}
				if verbs.PathItemObject.Get != nil {
					maps = append(maps, verbs.PathItemObject.Get.Responses)
				}
				if verbs.PathItemObject.Post != nil {
					maps = append(maps, verbs.PathItemObject.Post.Responses)
				}
				if verbs.PathItemObject.Put != nil {
					maps = append(maps, verbs.PathItemObject.Put.Responses)
				}
				if verbs.PathItemObject.Patch != nil {
					maps = append(maps, verbs.PathItemObject.Patch.Responses)
				}

				for k, v := range spb.Responses {
					for _, respMap := range maps {
						if _, ok := respMap[k]; ok {
							// Don't overwrite already existing Responses
							continue
						}
						respMap[k] = openapiResponseObject{
							Description: v.Description,
							Schema:      openapiSchemaFromProtoSchema(v.Schema, p.reg, customRefs, nil),
							Examples:    openapiExamplesFromProtoExamples(v.Examples),
						}
					}
				}
			}
		}

		if spb.Extensions != nil {
			exts, err := processExtensions(spb.Extensions)
			if err != nil {
				return nil, err
			}
			s.extensions = exts
		}

		if spb.Tags != nil {
			for _, v := range spb.Tags {
				newTag := openapiTagObject{}
				newTag.Name = v.Name
				newTag.Description = v.Description
				if p.reg.GetUseGoTemplate() {
					newTag.Description = goTemplateComments(newTag.Description, nil, p.reg)
				}
				if v.ExternalDocs != nil {
					newTag.ExternalDocs = &openapiExternalDocumentationObject{
						Description: v.ExternalDocs.Description,
						URL:         v.ExternalDocs.Url,
					}
					if p.reg.GetUseGoTemplate() {
						newTag.ExternalDocs.Description = goTemplateComments(v.ExternalDocs.Description, nil, p.reg)
					}
				}
				if v.Extensions != nil {
					exts, err := processExtensions(v.Extensions)
					if err != nil {
						return nil, err
					}
					newTag.extensions = exts
				}
				s.Tags = append(s.Tags, newTag)
			}
		}

		// Additional fields on the OpenAPI v2 spec's "OpenAPI" object
		// should be added here, once supported in the proto.
	}

	if !p.reg.GetDisableServiceTags() {
		s.Tags = mergeTags(s.Tags, renderServiceTags(p.Services, p.reg))
	}

	// Finally add any references added by users that aren't
	// otherwise rendered.
	if err := addCustomRefs(s.Definitions, p.reg, customRefs); err != nil {
		return nil, err
	}

	return &s, nil
}

func mergeTags(existingTags []openapiTagObject, tags []openapiTagObject) []openapiTagObject {
	for _, tag := range tags {
		matched := false
		for i, existingTag := range existingTags {
			if existingTag.Name == tag.Name {
				if existingTag.Description == "" {
					existingTags[i].Description = tag.Description
				}
				if existingTag.ExternalDocs == nil {
					existingTags[i].ExternalDocs = tag.ExternalDocs
				} else if tag.ExternalDocs != nil {
					if existingTag.ExternalDocs.Description == "" {
						existingTags[i].ExternalDocs.Description = tag.ExternalDocs.Description
					}
					if existingTag.ExternalDocs.URL == "" {
						existingTags[i].ExternalDocs.URL = tag.ExternalDocs.URL
					}
				}
				if existingTag.extensions == nil {
					existingTags[i].extensions = tag.extensions
				} else if tag.extensions != nil {
					for _, ext := range tag.extensions {
						matchedExt := false
						for _, existingExt := range existingTag.extensions {
							if existingExt.key == ext.key {
								matchedExt = true
								break
							}
						}
						if !matchedExt {
							existingTags[i].extensions = append(existingTags[i].extensions, ext)
						}
					}
				}
				matched = true
				break
			}
		}
		if !matched {
			existingTags = append(existingTags, tag)
		}
	}
	return existingTags
}

func processExtensions(inputExts map[string]*structpb.Value) ([]extension, error) {
	exts := make([]extension, 0, len(inputExts))
	for k, v := range inputExts {
		if !strings.HasPrefix(k, "x-") {
			return nil, fmt.Errorf("extension keys need to start with \"x-\": %q", k)
		}
		ext, err := (&protojson.MarshalOptions{Indent: "  "}).Marshal(v)
		if err != nil {
			return nil, err
		}
		exts = append(exts, extension{key: k, value: ext})
	}
	sort.Slice(exts, func(i, j int) bool { return exts[i].key < exts[j].key })
	return exts, nil
}

func validateHeaderTypeAndFormat(headerType, format string) error {
	// The type of the object. The value MUST be one of "string", "number", "integer", "boolean", or "array"
	// See: https://github.com/OAI/OpenAPI-Specification/blob/3.0.0/versions/2.0.md#headerObject
	// Note: currently not implementing array as we are only implementing this in the operation response context
	switch headerType {
	// the format property is an open string-valued property, and can have any value to support documentation needs
	// primary check for format is to ensure that the number/integer formats are extensions of the specified type
	// See: https://github.com/OAI/OpenAPI-Specification/blob/3.0.0/versions/2.0.md#dataTypeFormat
	case "string":
		return nil
	case "number":
		switch format {
		case "uint",
			"uint8",
			"uint16",
			"uint32",
			"uint64",
			"int",
			"int8",
			"int16",
			"int32",
			"int64",
			"float",
			"float32",
			"float64",
			"complex64",
			"complex128",
			"double",
			"byte",
			"rune",
			"uintptr",
			"":
			return nil
		default:
			return fmt.Errorf("the provided format %q is not a valid extension of the type %q", format, headerType)
		}
	case "integer":
		switch format {
		case "uint",
			"uint8",
			"uint16",
			"uint32",
			"uint64",
			"int",
			"int8",
			"int16",
			"int32",
			"int64",
			"":
			return nil
		default:
			return fmt.Errorf("the provided format %q is not a valid extension of the type %q", format, headerType)
		}
	case "boolean":
		return nil
	}
	return fmt.Errorf("the provided header type %q is not supported", headerType)
}

func validateDefaultValueTypeAndFormat(headerType string, defaultValue string, format string) error {
	switch headerType {
	case "string":
		if !isQuotedString(defaultValue) {
			return fmt.Errorf("the provided default value %q does not match provider type %q, or is not properly quoted with escaped quotations", defaultValue, headerType)
		}
		switch format {
		case "date-time":
			unquoteTime := strings.Trim(defaultValue, `"`)
			if _, err := time.Parse(time.RFC3339, unquoteTime); err != nil {
				return fmt.Errorf("the provided default value %q is not a valid RFC3339 date-time string", defaultValue)
			}
		case "date":
			const layoutRFC3339Date = "2006-01-02"
			unquoteDate := strings.Trim(defaultValue, `"`)
			if _, err := time.Parse(layoutRFC3339Date, unquoteDate); err != nil {
				return fmt.Errorf("the provided default value %q is not a valid RFC3339 date-time string", defaultValue)
			}
		}
	case "number":
		if err := isJSONNumber(defaultValue, headerType); err != nil {
			return err
		}
	case "integer":
		switch format {
		case "int32":
			if _, err := strconv.ParseInt(defaultValue, 0, 32); err != nil {
				return fmt.Errorf("the provided default value %q does not match provided format %q", defaultValue, format)
			}
		case "uint32":
			if _, err := strconv.ParseUint(defaultValue, 0, 32); err != nil {
				return fmt.Errorf("the provided default value %q does not match provided format %q", defaultValue, format)
			}
		case "int64":
			if _, err := strconv.ParseInt(defaultValue, 0, 64); err != nil {
				return fmt.Errorf("the provided default value %q does not match provided format %q", defaultValue, format)
			}
		case "uint64":
			if _, err := strconv.ParseUint(defaultValue, 0, 64); err != nil {
				return fmt.Errorf("the provided default value %q does not match provided format %q", defaultValue, format)
			}
		default:
			if _, err := strconv.ParseInt(defaultValue, 0, 64); err != nil {
				return fmt.Errorf("the provided default value %q does not match provided type %q", defaultValue, headerType)
			}
		}
	case "boolean":
		if !isBool(defaultValue) {
			return fmt.Errorf("the provided default value %q does not match provider type %q", defaultValue, headerType)
		}
	}
	return nil
}

func isQuotedString(s string) bool {
	return len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"'
}

func isJSONNumber(s string, t string) error {
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("the provided default value %q does not match provider type %q", s, t)
	}
	// Floating point values that cannot be represented as sequences of digits (such as Infinity and NaN) are not permitted.
	// See: https://tools.ietf.org/html/rfc4627#section-2.4
	if math.IsInf(val, 0) || math.IsNaN(val) {
		return fmt.Errorf("the provided number %q is not a valid JSON number", s)
	}

	return nil
}

func isBool(s string) bool {
	// Unable to use strconv.ParseBool because it returns truthy values https://golang.org/pkg/strconv/#example_ParseBool
	// per https://swagger.io/specification/v2/#data-types
	// type: boolean represents two values: true and false. Note that truthy and falsy values such as "true", "", 0 or null are not considered boolean values.
	return s == "true" || s == "false"
}

func processHeaders(inputHdrs map[string]*openapi_options.Header) (openapiHeadersObject, error) {
	hdrs := make(map[string]openapiHeaderObject, len(inputHdrs))
	for k, v := range inputHdrs {
		header := textproto.CanonicalMIMEHeaderKey(k)
		ret := openapiHeaderObject{
			Description: v.Description,
			Format:      v.Format,
			Pattern:     v.Pattern,
		}
		if err := validateHeaderTypeAndFormat(v.Type, v.Format); err != nil {
			return nil, err
		}
		ret.Type = v.Type
		if v.Default != "" {
			if err := validateDefaultValueTypeAndFormat(v.Type, v.Default, v.Format); err != nil {
				return nil, err
			}
			ret.Default = RawExample(v.Default)
		}
		hdrs[header] = ret
	}
	return hdrs, nil
}

func removeInternalComments(comment string) string {
	c := []string{}
	for len(comment) > 0 {
		open := strings.SplitN(comment, "(--", 2)
		if len(open) == 1 {
			c = append(c, open[0])
			break
		}
		ex := strings.TrimRight(open[0], " \t")
		// Trim only one line prior to all spaces
		switch {
		case strings.HasSuffix(ex, "\r\n"):
			ex = strings.TrimSuffix(ex, "\r\n")
		case strings.HasSuffix(ex, "\n"):
			ex = strings.TrimSuffix(ex, "\n")
		}
		if ex != "" {
			c = append(c, ex)
		}
		comment = open[1]

		close := strings.SplitN(comment, "--)", 2)
		if len(close) > 1 {
			comment = close[1]
		} else {
			break
		}
	}
	return strings.Join(c, "")
}

// updateOpenAPIDataFromComments updates a OpenAPI object based on a comment
// from the proto file.
//
// First paragraph of a comment is used for summary. Remaining paragraphs of
// a comment are used for description. If 'Summary' field is not present on
// the passed swaggerObject, the summary and description are joined by \n\n.
//
// If there is a field named 'Info', its 'Summary' and 'Description' fields
// will be updated instead.
//
// If there is no 'Summary', the same behavior will be attempted on 'Title',
// but only if the last character is not a period.
func updateOpenAPIDataFromComments(reg *descriptor.Registry, swaggerObject interface{}, data interface{}, comment string, isPackageObject bool) error {
	if len(comment) == 0 {
		return nil
	}

	// Checks whether the "ignore_comments" flag is set to true
	if reg.GetIgnoreComments() {
		return nil
	}

	// Checks whether the "remove_internal_comments" flag is set to true
	if reg.GetRemoveInternalComments() {
		comment = removeInternalComments(comment)
	}

	// Checks whether the "use_go_templates" flag is set to true
	if reg.GetUseGoTemplate() {
		comment = goTemplateComments(comment, data, reg)
	}

	// Figure out what to apply changes to.
	swaggerObjectValue := reflect.ValueOf(swaggerObject)
	infoObjectValue := swaggerObjectValue.Elem().FieldByName("Info")
	if !infoObjectValue.CanSet() {
		// No such field? Apply summary and description directly to
		// passed object.
		infoObjectValue = swaggerObjectValue.Elem()
	}

	// Figure out which properties to update.
	summaryValue := infoObjectValue.FieldByName("Summary")
	descriptionValue := infoObjectValue.FieldByName("Description")
	readOnlyValue := infoObjectValue.FieldByName("ReadOnly")

	if readOnlyValue.Kind() == reflect.Bool && readOnlyValue.CanSet() && strings.Contains(comment, "Output only.") {
		readOnlyValue.Set(reflect.ValueOf(true))
	}

	usingTitle := false
	if !summaryValue.CanSet() {
		summaryValue = infoObjectValue.FieldByName("Title")
		usingTitle = true
	}

	paragraphs := strings.Split(comment, paragraphDeliminator)

	// If there is a summary (or summary-equivalent) and it's empty, use the first
	// paragraph as summary, and the rest as description.
	if summaryValue.CanSet() {
		summary := strings.TrimSpace(paragraphs[0])
		description := strings.TrimSpace(strings.Join(paragraphs[1:], paragraphDeliminator))
		if !usingTitle || (len(summary) > 0 && summary[len(summary)-1] != '.') {
			// overrides the schema value only if it's empty
			// keep the comment precedence when updating the package definition
			if summaryValue.Len() == 0 || isPackageObject {
				summaryValue.Set(reflect.ValueOf(summary))
			}
			if len(description) > 0 {
				if !descriptionValue.CanSet() {
					return errors.New("encountered object type with a summary, but no description")
				}
				// overrides the schema value only if it's empty
				// keep the comment precedence when updating the package definition
				if descriptionValue.Len() == 0 || isPackageObject {
					descriptionValue.Set(reflect.ValueOf(description))
				}
			}
			return nil
		}
	}

	// There was no summary field on the swaggerObject. Try to apply the
	// whole comment into description if the OpenAPI object description is empty.
	if descriptionValue.CanSet() {
		if descriptionValue.Len() == 0 || isPackageObject {
			descriptionValue.Set(reflect.ValueOf(strings.Join(paragraphs, paragraphDeliminator)))
		}
		return nil
	}

	return errors.New("no description nor summary property")
}

func fieldProtoComments(reg *descriptor.Registry, msg *descriptor.Message, field *descriptor.Field) string {
	protoPath := protoPathIndex(reflect.TypeOf((*descriptorpb.DescriptorProto)(nil)), "Field")
	for i, f := range msg.Fields {
		if f == field {
			return protoComments(reg, msg.File, msg.Outers, "MessageType", int32(msg.Index), protoPath, int32(i))
		}
	}
	return ""
}

func enumValueProtoComments(reg *descriptor.Registry, enum *descriptor.Enum) string {
	protoPath := protoPathIndex(reflect.TypeOf((*descriptorpb.EnumDescriptorProto)(nil)), "Value")
	var comments []string
	for idx, value := range enum.GetValue() {
		if reg.GetOmitEnumDefaultValue() && value.GetNumber() == 0 {
			continue
		}
		if !isVisible(getEnumValueVisibilityOption(value), reg) {
			continue
		}
		name := value.GetName()
		if reg.GetEnumsAsInts() {
			name = strconv.Itoa(int(value.GetNumber()))
		}
		if str := protoComments(reg, enum.File, enum.Outers, "EnumType", int32(enum.Index), protoPath, int32(idx)); str != "" {
			comments = append(comments, name+": "+str)
		}
	}
	if len(comments) > 0 {
		return "- " + strings.Join(comments, "\n - ")
	}
	return ""
}

func protoComments(reg *descriptor.Registry, file *descriptor.File, outers []string, typeName string, typeIndex int32, fieldPaths ...int32) string {
	if file.SourceCodeInfo == nil {
		fmt.Fprintln(os.Stderr, file.GetName(), "descriptor.File should not contain nil SourceCodeInfo")
		return ""
	}

	outerPaths := make([]int32, len(outers))
	for i := range outers {
		location := ""
		if file.Package != nil {
			location = file.GetPackage()
		}

		msg, err := reg.LookupMsg(location, strings.Join(outers[:i+1], "."))
		if err != nil {
			panic(err)
		}
		outerPaths[i] = int32(msg.Index)
	}

	for _, loc := range file.SourceCodeInfo.Location {
		if !isProtoPathMatches(loc.Path, outerPaths, typeName, typeIndex, fieldPaths) {
			continue
		}
		comments := ""
		if loc.LeadingComments != nil {
			comments = strings.TrimRight(*loc.LeadingComments, "\n")
			comments = strings.TrimSpace(comments)
			// TODO(ivucica): this is a hack to fix "// " being interpreted as "//".
			// perhaps we should:
			// - split by \n
			// - determine if every (but first and last) line begins with " "
			// - trim every line only if that is the case
			// - join by \n
			comments = strings.ReplaceAll(comments, "\n ", "\n")
			comments = removeInternalComments(comments)
		}
		if loc.TrailingComments != nil {
			trailing := strings.TrimSpace(*loc.TrailingComments)
			if comments == "" {
				comments = trailing
			} else {
				comments += "\n\n" + trailing
			}
		}
		return comments
	}
	return ""
}

func goTemplateComments(comment string, data interface{}, reg *descriptor.Registry) string {
	var temp bytes.Buffer
	tpl, err := template.New("").Funcs(template.FuncMap{
		// Allows importing documentation from a file
		"import": func(name string) string {
			file, err := os.ReadFile(name)
			if err != nil {
				return err.Error()
			}
			// Runs template over imported file
			return goTemplateComments(string(file), data, reg)
		},
		// Grabs title and description from a field
		"fieldcomments": func(msg *descriptor.Message, field *descriptor.Field) string {
			return strings.ReplaceAll(fieldProtoComments(reg, msg, field), "\n", "<br>")
		},
		"arg": func(name string) string {
			if v, f := reg.GetGoTemplateArgs()[name]; f {
				return v
			}
			return fmt.Sprintf("goTemplateArg %s not found", name)
		},
	}).Parse(comment)
	if err != nil {
		// If there is an error parsing the templating insert the error as string in the comment
		// to make it easier to debug the template error
		return err.Error()
	}
	if err := tpl.Execute(&temp, data); err != nil {
		// If there is an error executing the templating insert the error as string in the comment
		// to make it easier to debug the error
		return err.Error()
	}
	return temp.String()
}

var (
	messageProtoPath = protoPathIndex(reflect.TypeOf((*descriptorpb.FileDescriptorProto)(nil)), "MessageType")
	nestedProtoPath  = protoPathIndex(reflect.TypeOf((*descriptorpb.DescriptorProto)(nil)), "NestedType")
	packageProtoPath = protoPathIndex(reflect.TypeOf((*descriptorpb.FileDescriptorProto)(nil)), "Package")
	serviceProtoPath = protoPathIndex(reflect.TypeOf((*descriptorpb.FileDescriptorProto)(nil)), "Service")
	methodProtoPath  = protoPathIndex(reflect.TypeOf((*descriptorpb.ServiceDescriptorProto)(nil)), "Method")
)

func isProtoPathMatches(paths []int32, outerPaths []int32, typeName string, typeIndex int32, fieldPaths []int32) bool {
	if typeName == "Package" && typeIndex == packageProtoPath {
		// path for package comments is just [2], and all the other processing
		// is too complex for it.
		if len(paths) == 0 || typeIndex != paths[0] {
			return false
		}
		return true
	}

	if len(paths) != len(outerPaths)*2+2+len(fieldPaths) {
		return false
	}

	if typeName == "Method" {
		if paths[0] != serviceProtoPath || paths[2] != methodProtoPath {
			return false
		}
		paths = paths[2:]
	} else {
		typeNameDescriptor := reflect.TypeOf((*descriptorpb.FileDescriptorProto)(nil))

		if len(outerPaths) > 0 {
			if paths[0] != messageProtoPath || paths[1] != outerPaths[0] {
				return false
			}
			paths = paths[2:]
			outerPaths = outerPaths[1:]

			for i, v := range outerPaths {
				if paths[i*2] != nestedProtoPath || paths[i*2+1] != v {
					return false
				}
			}
			paths = paths[len(outerPaths)*2:]

			if typeName == "MessageType" {
				typeName = "NestedType"
			}
			typeNameDescriptor = reflect.TypeOf((*descriptorpb.DescriptorProto)(nil))
		}

		if paths[0] != protoPathIndex(typeNameDescriptor, typeName) || paths[1] != typeIndex {
			return false
		}
		paths = paths[2:]
	}

	for i, v := range fieldPaths {
		if paths[i] != v {
			return false
		}
	}
	return true
}

// protoPathIndex returns a path component for google.protobuf.descriptor.SourceCode_Location.
//
// Specifically, it returns an id as generated from descriptor proto which
// can be used to determine what type the id following it in the path is.
// For example, if we are trying to locate comments related to a field named
// `Address` in a message named `Person`, the path will be:
//
//	[4, a, 2, b]
//
// While `a` gets determined by the order in which the messages appear in
// the proto file, and `b` is the field index specified in the proto
// file itself, the path actually needs to specify that `a` refers to a
// message and not, say, a service; and  that `b` refers to a field and not
// an option.
//
// protoPathIndex figures out the values 4 and 2 in the above example. Because
// messages are top level objects, the value of 4 comes from field id for
// `MessageType` inside `google.protobuf.descriptor.FileDescriptor` message.
// This field has a message type `google.protobuf.descriptor.DescriptorProto`.
// And inside message `DescriptorProto`, there is a field named `Field` with id
// 2.
//
// Some code generators seem to be hardcoding these values; this method instead
// interprets them from `descriptor.proto`-derived Go source as necessary.
func protoPathIndex(descriptorType reflect.Type, what string) int32 {
	field, ok := descriptorType.Elem().FieldByName(what)
	if !ok {
		panic(fmt.Errorf("could not find protobuf descriptor type id for %s", what))
	}
	pbtag := field.Tag.Get("protobuf")
	if pbtag == "" {
		panic(fmt.Errorf("no Go tag 'protobuf' on protobuf descriptor for %s", what))
	}
	path, err := strconv.ParseInt(strings.Split(pbtag, ",")[1], 10, 32)
	if err != nil {
		panic(fmt.Errorf("protobuf descriptor id for %s cannot be converted to a number: %s", what, err.Error()))
	}
	return int32(path)
}

// extractOperationOptionFromMethodDescriptor extracts the message of type
// openapi_options.Operation from a given proto method's descriptor.
func extractOperationOptionFromMethodDescriptor(meth *descriptorpb.MethodDescriptorProto) (*openapi_options.Operation, error) {
	if meth.Options == nil {
		return nil, nil
	}
	if !proto.HasExtension(meth.Options, openapi_options.E_Openapiv2Operation) {
		return nil, nil
	}
	ext := proto.GetExtension(meth.Options, openapi_options.E_Openapiv2Operation)
	opts, ok := ext.(*openapi_options.Operation)
	if !ok {
		return nil, fmt.Errorf("extension is %T; want an Operation", ext)
	}
	return opts, nil
}

// extractSchemaOptionFromMessageDescriptor extracts the message of type
// openapi_options.Schema from a given proto message's descriptor.
func extractSchemaOptionFromMessageDescriptor(msg *descriptorpb.DescriptorProto) (*openapi_options.Schema, error) {
	if msg.Options == nil {
		return nil, nil
	}
	if !proto.HasExtension(msg.Options, openapi_options.E_Openapiv2Schema) {
		return nil, nil
	}
	ext := proto.GetExtension(msg.Options, openapi_options.E_Openapiv2Schema)
	opts, ok := ext.(*openapi_options.Schema)
	if !ok {
		return nil, fmt.Errorf("extension is %T; want a Schema", ext)
	}
	return opts, nil
}

// extractEnumSchemaOptionFromEnumDescriptor extracts the message of type
// openapi_options.EnumSchema from a given proto enum's descriptor.
func extractEnumSchemaOptionFromEnumDescriptor(enum *descriptorpb.EnumDescriptorProto) (*openapi_options.EnumSchema, error) {
	if enum.Options == nil {
		return nil, nil
	}
	if !proto.HasExtension(enum.Options, openapi_options.E_Openapiv2Enum) {
		return nil, nil
	}
	ext := proto.GetExtension(enum.Options, openapi_options.E_Openapiv2Enum)
	opts, ok := ext.(*openapi_options.EnumSchema)
	if !ok {
		return nil, fmt.Errorf("extension is %T; want a EnumSchema", ext)
	}
	return opts, nil
}

// extractTagOptionFromServiceDescriptor extracts the tag of type
// openapi_options.Tag from a given proto service's descriptor.
func extractTagOptionFromServiceDescriptor(svc *descriptorpb.ServiceDescriptorProto) (*openapi_options.Tag, error) {
	if svc.Options == nil {
		return nil, nil
	}
	if !proto.HasExtension(svc.Options, openapi_options.E_Openapiv2Tag) {
		return nil, nil
	}
	ext := proto.GetExtension(svc.Options, openapi_options.E_Openapiv2Tag)
	opts, ok := ext.(*openapi_options.Tag)
	if !ok {
		return nil, fmt.Errorf("extension is %T; want a Tag", ext)
	}
	return opts, nil
}

// extractOpenAPIOptionFromFileDescriptor extracts the message of type
// openapi_options.OpenAPI from a given proto method's descriptor.
func extractOpenAPIOptionFromFileDescriptor(file *descriptorpb.FileDescriptorProto) (*openapi_options.Swagger, error) {
	if file.Options == nil {
		return nil, nil
	}
	if !proto.HasExtension(file.Options, openapi_options.E_Openapiv2Swagger) {
		return nil, nil
	}
	ext := proto.GetExtension(file.Options, openapi_options.E_Openapiv2Swagger)
	opts, ok := ext.(*openapi_options.Swagger)
	if !ok {
		return nil, fmt.Errorf("extension is %T; want a OpenAPI object", ext)
	}
	return opts, nil
}

func extractJSONSchemaFromFieldDescriptor(fd *descriptorpb.FieldDescriptorProto) (*openapi_options.JSONSchema, error) {
	if fd.Options == nil {
		return nil, nil
	}
	if !proto.HasExtension(fd.Options, openapi_options.E_Openapiv2Field) {
		return nil, nil
	}
	ext := proto.GetExtension(fd.Options, openapi_options.E_Openapiv2Field)
	opts, ok := ext.(*openapi_options.JSONSchema)
	if !ok {
		return nil, fmt.Errorf("extension is %T; want a JSONSchema object", ext)
	}
	return opts, nil
}

func extractFieldBehaviorFromFieldDescriptor(fd *descriptorpb.FieldDescriptorProto) ([]annotations.FieldBehavior, error) {
	if fd.Options == nil {
		return nil, nil
	}
	if !proto.HasExtension(fd.Options, annotations.E_FieldBehavior) {
		return nil, nil
	}
	ext := proto.GetExtension(fd.Options, annotations.E_FieldBehavior)
	opts, ok := ext.([]annotations.FieldBehavior)
	if !ok {
		return nil, fmt.Errorf("extension is %T; want a []FieldBehavior object", ext)
	}
	return opts, nil
}

func getFieldVisibilityOption(fd *descriptor.Field) *visibility.VisibilityRule {
	if fd.Options == nil {
		return nil
	}
	if !proto.HasExtension(fd.Options, visibility.E_FieldVisibility) {
		return nil
	}
	ext := proto.GetExtension(fd.Options, visibility.E_FieldVisibility)
	opts, ok := ext.(*visibility.VisibilityRule)
	if !ok {
		return nil
	}
	return opts
}

func getServiceVisibilityOption(fd *descriptor.Service) *visibility.VisibilityRule {
	if fd.Options == nil {
		return nil
	}
	if !proto.HasExtension(fd.Options, visibility.E_ApiVisibility) {
		return nil
	}
	ext := proto.GetExtension(fd.Options, visibility.E_ApiVisibility)
	opts, ok := ext.(*visibility.VisibilityRule)
	if !ok {
		return nil
	}
	return opts
}

func getMethodVisibilityOption(fd *descriptor.Method) *visibility.VisibilityRule {
	if fd.Options == nil {
		return nil
	}
	if !proto.HasExtension(fd.Options, visibility.E_MethodVisibility) {
		return nil
	}
	ext := proto.GetExtension(fd.Options, visibility.E_MethodVisibility)
	opts, ok := ext.(*visibility.VisibilityRule)
	if !ok {
		return nil
	}
	return opts
}

func getEnumValueVisibilityOption(fd *descriptorpb.EnumValueDescriptorProto) *visibility.VisibilityRule {
	if fd.Options == nil {
		return nil
	}
	if !proto.HasExtension(fd.Options, visibility.E_ValueVisibility) {
		return nil
	}
	ext := proto.GetExtension(fd.Options, visibility.E_ValueVisibility)
	opts, ok := ext.(*visibility.VisibilityRule)
	if !ok {
		return nil
	}
	return opts
}

func getMethodOpenAPIOption(reg *descriptor.Registry, meth *descriptor.Method) (*openapi_options.Operation, error) {
	opts, err := extractOperationOptionFromMethodDescriptor(meth.MethodDescriptorProto)
	if err != nil {
		return nil, err
	}
	if opts != nil {
		return opts, nil
	}
	opts, ok := reg.GetOpenAPIMethodOption(meth.FQMN())
	if !ok {
		return nil, nil
	}
	return opts, nil
}

func getMessageOpenAPIOption(reg *descriptor.Registry, msg *descriptor.Message) (*openapi_options.Schema, error) {
	opts, err := extractSchemaOptionFromMessageDescriptor(msg.DescriptorProto)
	if err != nil {
		return nil, err
	}
	if opts != nil {
		return opts, nil
	}
	opts, ok := reg.GetOpenAPIMessageOption(msg.FQMN())
	if !ok {
		return nil, nil
	}
	return opts, nil
}

func getEnumOpenAPIOption(reg *descriptor.Registry, enum *descriptor.Enum) (*openapi_options.EnumSchema, error) {
	opts, err := extractEnumSchemaOptionFromEnumDescriptor(enum.EnumDescriptorProto)
	if err != nil {
		return nil, err
	}
	return opts, nil
}

func getServiceOpenAPIOption(reg *descriptor.Registry, svc *descriptor.Service) (*openapi_options.Tag, error) {
	if opts, ok := reg.GetOpenAPIServiceOption(svc.FQSN()); ok {
		return opts, nil
	}
	opts, err := extractTagOptionFromServiceDescriptor(svc.ServiceDescriptorProto)
	if err != nil {
		return nil, err
	}
	return opts, nil
}

func getFileOpenAPIOption(reg *descriptor.Registry, file *descriptor.File) (*openapi_options.Swagger, error) {
	opts, err := extractOpenAPIOptionFromFileDescriptor(file.FileDescriptorProto)
	if err != nil {
		return nil, err
	}
	if opts != nil {
		return opts, nil
	}
	opts, ok := reg.GetOpenAPIFileOption(*file.Name)
	if !ok {
		return nil, nil
	}
	return opts, nil
}

func getFieldOpenAPIOption(reg *descriptor.Registry, fd *descriptor.Field) (*openapi_options.JSONSchema, error) {
	opts, err := extractJSONSchemaFromFieldDescriptor(fd.FieldDescriptorProto)
	if err != nil {
		return nil, err
	}
	if opts != nil {
		return opts, nil
	}
	opts, ok := reg.GetOpenAPIFieldOption(fd.FQFN())
	if !ok {
		return nil, nil
	}
	return opts, nil
}

func getFieldBehaviorOption(reg *descriptor.Registry, fd *descriptor.Field) ([]annotations.FieldBehavior, error) {
	opts, err := extractFieldBehaviorFromFieldDescriptor(fd.FieldDescriptorProto)
	if err != nil {
		return nil, err
	}
	if opts != nil {
		return opts, nil
	}
	return opts, nil
}

func protoJSONSchemaToOpenAPISchemaCore(j *openapi_options.JSONSchema, reg *descriptor.Registry, refs refMap) schemaCore {
	ret := schemaCore{}

	if j.GetRef() != "" {
		openapiName, ok := fullyQualifiedNameToOpenAPIName(j.GetRef(), reg)
		if ok {
			ret.Ref = "#/definitions/" + openapiName
			if refs != nil {
				refs[j.GetRef()] = struct{}{}
			}
		} else {
			ret.Ref += j.GetRef()
		}
	} else {
		f, t := protoJSONSchemaTypeToFormat(j.GetType())
		ret.Format = f
		ret.Type = t
	}

	return ret
}

func updateswaggerObjectFromJSONSchema(s *openapiSchemaObject, j *openapi_options.JSONSchema, reg *descriptor.Registry, data interface{}) {
	s.Title = j.GetTitle()
	s.Description = j.GetDescription()
	if reg.GetUseGoTemplate() {
		s.Title = goTemplateComments(s.Title, data, reg)
		s.Description = goTemplateComments(s.Description, data, reg)
	}
	if s.Type == "array" {
		s.Items.MaxLength = j.GetMaxLength()
		s.Items.MinLength = j.GetMinLength()
		s.Items.Pattern = j.GetPattern()
		s.Items.Default = j.GetDefault()
		s.Items.MaxProperties = j.GetMaxProperties()
		s.Items.MinProperties = j.GetMinProperties()
		s.Items.Required = j.GetRequired()
		s.Items.Minimum = j.GetMinimum()
		s.Items.Maximum = j.GetMaximum()
		s.Items.ReadOnly = j.GetReadOnly()
		s.Items.MultipleOf = j.GetMultipleOf()
		s.Items.ExclusiveMaximum = j.GetExclusiveMaximum()
		s.Items.ExclusiveMinimum = j.GetExclusiveMinimum()
		s.Items.Enum = j.GetEnum()

		if j.GetDefault() == "" {
			s.Items.Default = nil
		}
		if len(j.GetEnum()) == 0 {
			s.Items.Enum = nil
		}
		if j.GetFormat() != "" {
			s.Items.Format = j.GetFormat()
		}
	} else {
		s.MaxLength = j.GetMaxLength()
		s.MinLength = j.GetMinLength()
		s.Pattern = j.GetPattern()
		s.Default = j.GetDefault()
		s.MaxProperties = j.GetMaxProperties()
		s.MinProperties = j.GetMinProperties()
		s.Required = j.GetRequired()
		s.Minimum = j.GetMinimum()
		s.Maximum = j.GetMaximum()
		s.ReadOnly = j.GetReadOnly()
		s.MultipleOf = j.GetMultipleOf()
		s.ExclusiveMaximum = j.GetExclusiveMaximum()
		s.ExclusiveMinimum = j.GetExclusiveMinimum()
		s.Enum = j.GetEnum()

		if j.GetDefault() == "" {
			s.Default = nil
		}
		if len(j.GetEnum()) == 0 {
			s.Enum = nil
		}
		if j.GetFormat() != "" {
			s.Format = j.GetFormat()
		}
	}
	s.UniqueItems = j.GetUniqueItems()
	s.MaxItems = j.GetMaxItems()
	s.MinItems = j.GetMinItems()

	if j.GetExtensions() != nil {
		exts, err := processExtensions(j.GetExtensions())
		if err != nil {
			panic(err)
		}
		s.extensions = exts
	}
	if overrideType := j.GetType(); len(overrideType) > 0 {
		s.Type = strings.ToLower(overrideType[0].String())
	}
	if j.GetExample() != "" {
		s.Example = RawExample(j.GetExample())
	}
}

func updateSwaggerObjectFromFieldBehavior(s *openapiSchemaObject, j []annotations.FieldBehavior, reg *descriptor.Registry, field *descriptor.Field) {
	required := false
	if reg.GetUseProto3FieldSemantics() {
		required = !field.GetProto3Optional() && field.OneofIndex == nil
	}
	for _, fb := range j {
		switch fb {
		case annotations.FieldBehavior_REQUIRED:
			required = true
		case annotations.FieldBehavior_OUTPUT_ONLY:
			s.ReadOnly = true
		case annotations.FieldBehavior_FIELD_BEHAVIOR_UNSPECIFIED:
		case annotations.FieldBehavior_OPTIONAL:
			required = false
		case annotations.FieldBehavior_INPUT_ONLY:
			// OpenAPI v3 supports a writeOnly property, but this is not supported in Open API v2
		case annotations.FieldBehavior_IMMUTABLE:
		}
	}
	if required {
		if reg.GetUseJSONNamesForFields() {
			s.Required = append(s.Required, *field.JsonName)
		} else {
			s.Required = append(s.Required, *field.Name)
		}
	}
}

func openapiSchemaFromProtoEnumSchema(s *openapi_options.EnumSchema, reg *descriptor.Registry, refs refMap, data interface{}) openapiSchemaObject {
	ret := openapiSchemaObject{
		ExternalDocs: protoExternalDocumentationToOpenAPIExternalDocumentation(s.GetExternalDocs(), reg, data),
	}
	jsonSchema := &openapi_options.JSONSchema{
		Ref:         s.Ref,
		Title:       s.Title,
		Extensions:  s.Extensions,
		Description: s.Description,
		Default:     s.Default,
		ReadOnly:    s.ReadOnly,
		Example:     s.Example,
	}
	ret.schemaCore = protoJSONSchemaToOpenAPISchemaCore(jsonSchema, reg, refs)
	updateswaggerObjectFromJSONSchema(&ret, jsonSchema, reg, data)
	return ret
}

func openapiSchemaFromProtoSchema(s *openapi_options.Schema, reg *descriptor.Registry, refs refMap, data interface{}) openapiSchemaObject {
	ret := openapiSchemaObject{
		ExternalDocs: protoExternalDocumentationToOpenAPIExternalDocumentation(s.GetExternalDocs(), reg, data),
	}

	ret.schemaCore = protoJSONSchemaToOpenAPISchemaCore(s.GetJsonSchema(), reg, refs)
	updateswaggerObjectFromJSONSchema(&ret, s.GetJsonSchema(), reg, data)

	if s != nil && s.Example != "" {
		ret.Example = RawExample(s.Example)
	}

	return ret
}

func openapiExamplesFromProtoExamples(in map[string]string) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for mimeType, exampleStr := range in {
		switch mimeType {
		case "application/json":
			// JSON example objects are rendered raw.
			out[mimeType] = RawExample(exampleStr)
		default:
			// All other mimetype examples are rendered as strings.
			out[mimeType] = exampleStr
		}
	}
	return out
}

func protoJSONSchemaTypeToFormat(in []openapi_options.JSONSchema_JSONSchemaSimpleTypes) (string, string) {
	if len(in) == 0 {
		return "", ""
	}

	// Can't support more than 1 type, just return the first element.
	// This is due to an inconsistency in the design of the openapiv2 proto
	// and that used in schemaCore. schemaCore uses the v3 definition of types,
	// which only allows a single string, while the openapiv2 proto uses the OpenAPI v2
	// definition, which defers to the JSON schema definition, which allows a string or an array.
	// Sources:
	// https://swagger.io/specification/#itemsObject
	// https://tools.ietf.org/html/draft-fge-json-schema-validation-00#section-5.5.2
	switch in[0] {
	case openapi_options.JSONSchema_UNKNOWN, openapi_options.JSONSchema_NULL:
		return "", ""
	case openapi_options.JSONSchema_OBJECT:
		return "object", ""
	case openapi_options.JSONSchema_ARRAY:
		return "array", ""
	case openapi_options.JSONSchema_BOOLEAN:
		// NOTE: in OpenAPI specification, format should be empty on boolean type
		return "boolean", ""
	case openapi_options.JSONSchema_INTEGER:
		return "integer", "int32"
	case openapi_options.JSONSchema_NUMBER:
		return "number", "double"
	case openapi_options.JSONSchema_STRING:
		// NOTE: in OpenAPI specification, format should be empty on string type
		return "string", ""
	default:
		// Maybe panic?
		return "", ""
	}
}

func protoExternalDocumentationToOpenAPIExternalDocumentation(in *openapi_options.ExternalDocumentation, reg *descriptor.Registry, data interface{}) *openapiExternalDocumentationObject {
	if in == nil {
		return nil
	}

	if reg.GetUseGoTemplate() {
		in.Description = goTemplateComments(in.Description, data, reg)
	}

	return &openapiExternalDocumentationObject{
		Description: in.Description,
		URL:         in.Url,
	}
}

func addCustomRefs(d openapiDefinitionsObject, reg *descriptor.Registry, refs refMap) error {
	if len(refs) == 0 {
		return nil
	}
	msgMap := make(messageMap)
	enumMap := make(enumMap)
	for ref := range refs {
		swgName, swgOk := fullyQualifiedNameToOpenAPIName(ref, reg)
		if !swgOk {
			grpclog.Errorf("can't resolve OpenAPI name from CustomRef %q", ref)
			continue
		}
		if _, ok := d[swgName]; ok {
			// Skip already existing definitions
			delete(refs, ref)
			continue
		}
		msg, err := reg.LookupMsg("", ref)
		if err == nil {
			msgMap[swgName] = msg
			continue
		}
		enum, err := reg.LookupEnum("", ref)
		if err == nil {
			enumMap[swgName] = enum
			continue
		}

		// ?? Should be either enum or msg
	}
	if err := renderMessagesAsDefinition(msgMap, d, reg, refs, nil); err != nil {
		return err
	}
	renderEnumerationsAsDefinition(enumMap, d, reg, refs)

	// Run again in case any new refs were added
	return addCustomRefs(d, reg, refs)
}

func lowerCamelCase(fieldName string, fields []*descriptor.Field, msgs []*descriptor.Message) string {
	for _, oneField := range fields {
		if oneField.GetName() == fieldName {
			return oneField.GetJsonName()
		}
	}
	messageNameToFieldsToJSONName := make(map[string]map[string]string, len(msgs))
	fieldNameToType := make(map[string]string)
	for _, msg := range msgs {
		fieldNameToJSONName := make(map[string]string)
		for _, oneField := range msg.GetField() {
			fieldNameToJSONName[oneField.GetName()] = oneField.GetJsonName()
			fieldNameToType[oneField.GetName()] = oneField.GetTypeName()
		}
		messageNameToFieldsToJSONName[msg.GetName()] = fieldNameToJSONName
	}
	if strings.Contains(fieldName, ".") {
		fieldNames := strings.Split(fieldName, ".")
		fieldNamesWithCamelCase := make([]string, 0)
		for i := 0; i < len(fieldNames)-1; i++ {
			fieldNamesWithCamelCase = append(fieldNamesWithCamelCase, casing.JSONCamelCase(fieldNames[i]))
		}
		prefix := strings.Join(fieldNamesWithCamelCase, ".")
		reservedJSONName := getReservedJSONName(fieldName, messageNameToFieldsToJSONName, fieldNameToType)
		if reservedJSONName != "" {
			return prefix + "." + reservedJSONName
		}
	}
	return casing.JSONCamelCase(fieldName)
}

func getReservedJSONName(fieldName string, messageNameToFieldsToJSONName map[string]map[string]string, fieldNameToType map[string]string) string {
	if len(strings.Split(fieldName, ".")) == 2 {
		fieldNames := strings.Split(fieldName, ".")
		firstVariable := fieldNames[0]
		firstType := fieldNameToType[firstVariable]
		firstTypeShortNames := strings.Split(firstType, ".")
		firstTypeShortName := firstTypeShortNames[len(firstTypeShortNames)-1]
		return messageNameToFieldsToJSONName[firstTypeShortName][fieldNames[1]]
	}
	fieldNames := strings.Split(fieldName, ".")
	return getReservedJSONName(strings.Join(fieldNames[1:], "."), messageNameToFieldsToJSONName, fieldNameToType)
}

func find(a []string, x string) int {
	// This is a linear search but we are dealing with a small number of fields
	for i, n := range a {
		if x == n {
			return i
		}
	}
	return -1
}

// Make a deep copy of the outer parameters that has paramName as the first component,
// but remove the first component of the field path.
func subPathParams(paramName string, outerParams []descriptor.Parameter) []descriptor.Parameter {
	var innerParams []descriptor.Parameter
	for _, p := range outerParams {
		if len(p.FieldPath) > 1 && p.FieldPath[0].Name == paramName {
			subParam := descriptor.Parameter{
				FieldPath: p.FieldPath[1:],
				Target:    p.Target,
				Method:    p.Method,
			}
			innerParams = append(innerParams, subParam)
		}
	}
	return innerParams
}

func getFieldConfiguration(reg *descriptor.Registry, fd *descriptor.Field) *openapi_options.JSONSchema_FieldConfiguration {
	if j, err := getFieldOpenAPIOption(reg, fd); err == nil {
		return j.GetFieldConfiguration()
	}
	return nil
}
