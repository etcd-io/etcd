package genopenapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/internal/descriptor"
	gen "github.com/grpc-ecosystem/grpc-gateway/v2/internal/generator"
	openapioptions "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	"go.yaml.in/yaml/v3"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/pluginpb"
)

var errNoTargetService = errors.New("no target service defined in the file")

type generator struct {
	reg    *descriptor.Registry
	format Format
}

type wrapper struct {
	fileName string
	swagger  *openapiSwaggerObject
}

type GeneratorOptions struct {
	Registry       *descriptor.Registry
	RecursiveDepth int
}

// New returns a new generator which generates grpc gateway files.
func New(reg *descriptor.Registry, format Format) gen.Generator {
	return &generator{
		reg:    reg,
		format: format,
	}
}

// Merge a lot of OpenAPI file (wrapper) to single one OpenAPI file
func mergeTargetFile(targets []*wrapper, mergeFileName string) *wrapper {
	var mergedTarget *wrapper
	for _, f := range targets {
		if mergedTarget == nil {
			mergedTarget = &wrapper{
				fileName: mergeFileName,
				swagger:  f.swagger,
			}
		} else {
			for k, v := range f.swagger.Definitions {
				mergedTarget.swagger.Definitions[k] = v
			}
			for k, v := range f.swagger.SecurityDefinitions {
				mergedTarget.swagger.SecurityDefinitions[k] = v
			}
			copy(mergedTarget.swagger.Paths, f.swagger.Paths)
			mergedTarget.swagger.Security = append(mergedTarget.swagger.Security, f.swagger.Security...)
		}
	}
	return mergedTarget
}

// Q: What's up with the alias types here?
// A: We don't want to completely override how these structs are marshaled into
// JSON, we only want to add fields (see below, extensionMarshalJSON).
// An infinite recursion would happen if we'd call json.Marshal on the struct
// that has swaggerObject as an embedded field. To avoid that, we'll create
// type aliases, and those don't have the custom MarshalJSON methods defined
// on them. See http://choly.ca/post/go-json-marshalling/ (or, if it ever
// goes away, use
// https://web.archive.org/web/20190806073003/http://choly.ca/post/go-json-marshalling/).
func (so openapiSwaggerObject) MarshalJSON() ([]byte, error) {
	type alias openapiSwaggerObject
	return extensionMarshalJSON(alias(so), so.extensions)
}

// MarshalYAML implements yaml.Marshaler interface.
//
// It is required in order to pass extensions inline.
//
// Example:
//
//	extensions: {x-key: x-value}
//	type: string
//
// It will be rendered as:
//
//	x-key: x-value
//	type: string
//
// Use generics when the project will be upgraded to go 1.18+.
func (so openapiSwaggerObject) MarshalYAML() (interface{}, error) {
	type Alias openapiSwaggerObject

	return struct {
		Extension map[string]interface{} `yaml:",inline"`
		Alias     `yaml:",inline"`
	}{
		Extension: extensionsToMap(so.extensions),
		Alias:     Alias(so),
	}, nil
}

// Custom json marshaller for openapiPathsObject. Ensures
// openapiPathsObject is marshalled into expected format in generated
// swagger.json.
func (po openapiPathsObject) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("{")
	for i, pd := range po {
		if i != 0 {
			buf.WriteString(",")
		}
		// marshal key
		key, err := json.Marshal(pd.Path)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteString(":")
		// marshal value
		val, err := json.Marshal(pd.PathItemObject)
		if err != nil {
			return nil, err
		}
		buf.Write(val)
	}

	buf.WriteString("}")
	return buf.Bytes(), nil
}

// Custom yaml marshaller for openapiPathsObject. Ensures
// openapiPathsObject is marshalled into expected format in generated
// swagger.yaml.
func (po openapiPathsObject) MarshalYAML() (interface{}, error) {
	var pathObjectNode yaml.Node
	pathObjectNode.Kind = yaml.MappingNode

	for _, pathData := range po {
		var pathNode yaml.Node

		pathNode.SetString(pathData.Path)
		pathItemObjectNode, err := pathData.PathItemObject.toYAMLNode()
		if err != nil {
			return nil, err
		}
		pathObjectNode.Content = append(pathObjectNode.Content, &pathNode, pathItemObjectNode)
	}

	return pathObjectNode, nil
}

// We can simplify this implementation once the go-yaml bug is resolved. See: https://github.com/go-yaml/yaml/issues/643.
//
//	func (pio *openapiPathItemObject) toYAMLNode() (*yaml.Node, error) {
//		var node yaml.Node
//		if err := node.Encode(pio); err != nil {
//			return nil, err
//		}
//		return &node, nil
//	}
func (pio *openapiPathItemObject) toYAMLNode() (*yaml.Node, error) {
	var doc yaml.Node
	var buf bytes.Buffer
	ec := yaml.NewEncoder(&buf)
	ec.SetIndent(2)
	if err := ec.Encode(pio); err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(buf.Bytes(), &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, errors.New("unexpected number of yaml nodes")
	}
	return doc.Content[0], nil
}

func (so openapiInfoObject) MarshalJSON() ([]byte, error) {
	type alias openapiInfoObject
	return extensionMarshalJSON(alias(so), so.extensions)
}

func (so openapiInfoObject) MarshalYAML() (interface{}, error) {
	type Alias openapiInfoObject

	return struct {
		Extension map[string]interface{} `yaml:",inline"`
		Alias     `yaml:",inline"`
	}{
		Extension: extensionsToMap(so.extensions),
		Alias:     Alias(so),
	}, nil
}

func (so openapiSecuritySchemeObject) MarshalJSON() ([]byte, error) {
	type alias openapiSecuritySchemeObject
	return extensionMarshalJSON(alias(so), so.extensions)
}

func (so openapiSecuritySchemeObject) MarshalYAML() (interface{}, error) {
	type Alias openapiSecuritySchemeObject

	return struct {
		Extension map[string]interface{} `yaml:",inline"`
		Alias     `yaml:",inline"`
	}{
		Extension: extensionsToMap(so.extensions),
		Alias:     Alias(so),
	}, nil
}

func (so openapiOperationObject) MarshalJSON() ([]byte, error) {
	type alias openapiOperationObject
	return extensionMarshalJSON(alias(so), so.extensions)
}

func (so openapiOperationObject) MarshalYAML() (interface{}, error) {
	type Alias openapiOperationObject

	return struct {
		Extension map[string]interface{} `yaml:",inline"`
		Alias     `yaml:",inline"`
	}{
		Extension: extensionsToMap(so.extensions),
		Alias:     Alias(so),
	}, nil
}

func (so openapiResponseObject) MarshalJSON() ([]byte, error) {
	type alias openapiResponseObject
	return extensionMarshalJSON(alias(so), so.extensions)
}

func (so openapiResponseObject) MarshalYAML() (interface{}, error) {
	type Alias openapiResponseObject

	return struct {
		Extension map[string]interface{} `yaml:",inline"`
		Alias     `yaml:",inline"`
	}{
		Extension: extensionsToMap(so.extensions),
		Alias:     Alias(so),
	}, nil
}

func (so openapiSchemaObject) MarshalJSON() ([]byte, error) {
	type alias openapiSchemaObject
	return extensionMarshalJSON(alias(so), so.extensions)
}

func (so openapiSchemaObject) MarshalYAML() (interface{}, error) {
	type Alias openapiSchemaObject

	return struct {
		Extension map[string]interface{} `yaml:",inline"`
		Alias     `yaml:",inline"`
	}{
		Extension: extensionsToMap(so.extensions),
		Alias:     Alias(so),
	}, nil
}

func (so openapiParameterObject) MarshalJSON() ([]byte, error) {
	type alias openapiParameterObject
	return extensionMarshalJSON(alias(so), so.extensions)
}

func (so openapiParameterObject) MarshalYAML() (interface{}, error) {
	type Alias openapiParameterObject

	return struct {
		Extension map[string]interface{} `yaml:",inline"`
		Alias     `yaml:",inline"`
	}{
		Extension: extensionsToMap(so.extensions),
		Alias:     Alias(so),
	}, nil
}

func (so openapiTagObject) MarshalJSON() ([]byte, error) {
	type alias openapiTagObject
	return extensionMarshalJSON(alias(so), so.extensions)
}

func (so openapiTagObject) MarshalYAML() (interface{}, error) {
	type Alias openapiTagObject

	return struct {
		Extension map[string]interface{} `yaml:",inline"`
		Alias     `yaml:",inline"`
	}{
		Extension: extensionsToMap(so.extensions),
		Alias:     Alias(so),
	}, nil
}

func extensionMarshalJSON(so interface{}, extensions []extension) ([]byte, error) {
	// To append arbitrary keys to the struct we'll render into json,
	// we're creating another struct that embeds the original one, and
	// its extra fields:
	//
	// The struct will look like
	// struct {
	//   *openapiCore
	//   XGrpcGatewayFoo json.RawMessage `json:"x-grpc-gateway-foo"`
	//   XGrpcGatewayBar json.RawMessage `json:"x-grpc-gateway-bar"`
	// }
	// and thus render into what we want -- the JSON of openapiCore with the
	// extensions appended.
	fields := []reflect.StructField{
		{ // embedded
			Name:      "Embedded",
			Type:      reflect.TypeOf(so),
			Anonymous: true,
		},
	}
	for _, ext := range extensions {
		fields = append(fields, reflect.StructField{
			Name: fieldName(ext.key),
			Type: reflect.TypeOf(ext.value),
			Tag:  reflect.StructTag(fmt.Sprintf("json:\"%s\"", ext.key)),
		})
	}

	t := reflect.StructOf(fields)
	s := reflect.New(t).Elem()
	s.Field(0).Set(reflect.ValueOf(so))
	for _, ext := range extensions {
		s.FieldByName(fieldName(ext.key)).Set(reflect.ValueOf(ext.value))
	}
	return json.Marshal(s.Interface())
}

// encodeOpenAPI converts OpenAPI file obj to pluginpb.CodeGeneratorResponse_File
func encodeOpenAPI(file *wrapper, format Format) (*descriptor.ResponseFile, error) {
	var contentBuf bytes.Buffer
	enc, err := format.NewEncoder(&contentBuf)
	if err != nil {
		return nil, err
	}

	if err := enc.Encode(*file.swagger); err != nil {
		return nil, err
	}

	name := file.fileName
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	output := fmt.Sprintf("%s.swagger."+string(format), base)
	return &descriptor.ResponseFile{
		CodeGeneratorResponse_File: &pluginpb.CodeGeneratorResponse_File{
			Name:    proto.String(output),
			Content: proto.String(contentBuf.String()),
		},
	}, nil
}

func deprecateFieldsAndMethods(file *descriptor.File) {
	for _, msg := range file.Messages {
		for _, field := range msg.GetField() {
			if field.Options == nil {
				field.Options = &descriptorpb.FieldOptions{}
			}
			field.Options.Deprecated = proto.Bool(true)
		}
	}
	for _, svc := range file.Services {
		for _, method := range svc.GetMethod() {
			if method.Options == nil {
				method.Options = &descriptorpb.MethodOptions{}
			}
			method.Options.Deprecated = proto.Bool(true)
		}
	}
}

func (g *generator) Generate(targets []*descriptor.File) ([]*descriptor.ResponseFile, error) {
	var files []*descriptor.ResponseFile
	for _, f := range targets {
		// Because of how the generator merges definitions, it is simpler to deprecate field and methods here if the file is deprecated
		if opts := f.GetOptions(); opts != nil && opts.GetDeprecated() {
			deprecateFieldsAndMethods(f)
		}
	}
	if g.reg.IsAllowMerge() {
		var mergedTarget *descriptor.File
		// try to find proto leader
		for _, f := range targets {
			if proto.HasExtension(f.Options, openapioptions.E_Openapiv2Swagger) {
				mergedTarget = f
				break
			}
		}
		// merge protos to leader
		for _, f := range targets {
			if mergedTarget == nil {
				mergedTarget = f
			} else if mergedTarget != f {
				mergedTarget.Enums = append(mergedTarget.Enums, f.Enums...)
				mergedTarget.Messages = append(mergedTarget.Messages, f.Messages...)
				mergedTarget.Services = append(mergedTarget.Services, f.Services...)
			}
		}

		targets = nil
		targets = append(targets, mergedTarget)
	}

	var openapis []*wrapper
	for _, file := range targets {
		if grpclog.V(1) {
			grpclog.Infof("Processing %s", file.GetName())
		}
		swagger, err := applyTemplate(param{File: file, reg: g.reg})
		if errors.Is(err, errNoTargetService) {
			if grpclog.V(1) {
				grpclog.Infof("%s: %v", file.GetName(), err)
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		openapis = append(openapis, &wrapper{
			fileName: file.GetName(),
			swagger:  swagger,
		})
	}

	if g.reg.IsAllowMerge() {
		targetOpenAPI := mergeTargetFile(openapis, g.reg.GetMergeFileName())
		if !g.reg.IsPreserveRPCOrder() {
			targetOpenAPI.swagger.sortPathsAlphabetically()
		}
		f, err := encodeOpenAPI(targetOpenAPI, g.format)
		if err != nil {
			return nil, fmt.Errorf("failed to encode OpenAPI for %s: %w", g.reg.GetMergeFileName(), err)
		}
		files = append(files, f)
		if grpclog.V(1) {
			grpclog.Infof("New OpenAPI file will emit")
		}
	} else {
		for _, file := range openapis {
			if !g.reg.IsPreserveRPCOrder() {
				file.swagger.sortPathsAlphabetically()
			}
			f, err := encodeOpenAPI(file, g.format)
			if err != nil {
				return nil, fmt.Errorf("failed to encode OpenAPI for %s: %w", file.fileName, err)
			}
			files = append(files, f)
			if grpclog.V(1) {
				grpclog.Infof("New OpenAPI file will emit")
			}
		}
	}
	return files, nil
}

func (so openapiSwaggerObject) sortPathsAlphabetically() {
	sort.Slice(so.Paths, func(i, j int) bool {
		return so.Paths[i].Path < so.Paths[j].Path
	})
}

// AddErrorDefs Adds google.rpc.Status and google.protobuf.Any
// to registry (used for error-related API responses)
func AddErrorDefs(reg *descriptor.Registry) error {
	// load internal protos
	any := protodesc.ToFileDescriptorProto((&anypb.Any{}).ProtoReflect().Descriptor().ParentFile())
	any.SourceCodeInfo = new(descriptorpb.SourceCodeInfo)
	status := protodesc.ToFileDescriptorProto((&statuspb.Status{}).ProtoReflect().Descriptor().ParentFile())
	status.SourceCodeInfo = new(descriptorpb.SourceCodeInfo)
	return reg.Load(&pluginpb.CodeGeneratorRequest{
		ProtoFile: []*descriptorpb.FileDescriptorProto{
			any,
			status,
		},
	})
}

func extensionsToMap(extensions []extension) map[string]interface{} {
	m := make(map[string]interface{}, len(extensions))

	for _, v := range extensions {
		m[v.key] = RawExample(v.value)
	}

	return m
}
