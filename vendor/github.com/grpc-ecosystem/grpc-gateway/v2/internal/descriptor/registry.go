package descriptor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/internal/codegenerator"
	"github.com/grpc-ecosystem/grpc-gateway/v2/internal/descriptor/openapiconfig"
	"github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// Registry is a registry of information extracted from pluginpb.CodeGeneratorRequest.
type Registry struct {
	// msgs is a mapping from fully-qualified message name to descriptor
	msgs map[string]*Message

	// enums is a mapping from fully-qualified enum name to descriptor
	enums map[string]*Enum

	// files is a mapping from file path to descriptor
	files map[string]*File

	// meths is a mapping from fully-qualified method name to descriptor
	meths map[string]*Method

	// prefix is a prefix to be inserted to golang package paths generated from proto package names.
	prefix string

	// pkgMap is a user-specified mapping from file path to proto package.
	pkgMap map[string]string

	// pkgAliases is a mapping from package aliases to package paths in go which are already taken.
	pkgAliases map[string]string

	// allowDeleteBody permits http delete methods to have a body
	allowDeleteBody bool

	// externalHttpRules is a mapping from fully qualified service method names to additional HttpRules applicable besides the ones found in annotations.
	externalHTTPRules map[string][]*annotations.HttpRule

	// allowMerge generation one OpenAPI file out of multiple protos
	allowMerge bool

	// mergeFileName target OpenAPI file name after merge
	mergeFileName string

	// includePackageInTags controls whether the package name defined in the `package` directive
	// in the proto file can be prepended to the gRPC service name in the `Tags` field of every operation.
	includePackageInTags bool

	// repeatedPathParamSeparator specifies how path parameter repeated fields are separated
	repeatedPathParamSeparator repeatedFieldSeparator

	// useJSONNamesForFields if true json tag name is used for generating fields in OpenAPI definitions,
	// otherwise the original proto name is used. It's helpful for synchronizing the OpenAPI definition
	// with gRPC-Gateway response, if it uses json tags for marshaling.
	useJSONNamesForFields bool

	// useProto3FieldSemantics if true proto3 field semantics are used for generating fields in OpenAPI definitions.
	useProto3FieldSemantics bool

	// openAPINamingStrategy is the naming strategy to use for assigning OpenAPI field and parameter names. This can be one of the following:
	// - `legacy`: use the legacy naming strategy from protoc-gen-swagger, that generates unique but not necessarily
	//             maximally concise names. Components are concatenated directly, e.g., `MyOuterMessageMyNestedMessage`.
	// - `simple`: use a simple heuristic for generating unique and concise names. Components are concatenated using
	//             dots as a separator, e.g., `MyOuterMessage.MyNestedMessage` (if `MyNestedMessage` alone is unique,
	//             `MyNestedMessage` will be used as the OpenAPI name).
	// - `fqn`:    always use the fully-qualified name of the proto message (leading dot removed) as the OpenAPI
	//             name.
	openAPINamingStrategy string

	// visibilityRestrictionSelectors is a map of selectors for `google.api.VisibilityRule`s that will be included in the OpenAPI output.
	visibilityRestrictionSelectors map[string]bool

	// useGoTemplate determines whether you want to use GO templates
	// in your protofile comments
	useGoTemplate bool

	// goTemplateArgs specifies a list of key value pair inputs to be displayed in Go templates
	goTemplateArgs map[string]string

	// ignoreComments determines whether all protofile comments should be excluded from output
	ignoreComments bool

	// removeInternalComments determines whether to remove substrings in comments that begin with
	// `(--` and end with `--)` as specified in https://google.aip.dev/192#internal-comments.
	removeInternalComments bool

	// enumsAsInts render enum as integer, as opposed to string
	enumsAsInts bool

	// omitEnumDefaultValue omits default value of enum
	omitEnumDefaultValue bool

	// disableDefaultErrors disables the generation of the default error types.
	// This is useful for users who have defined custom error handling.
	disableDefaultErrors bool

	// simpleOperationIDs removes the service prefix from the generated
	// operationIDs. This risks generating duplicate operationIDs.
	simpleOperationIDs bool

	standalone bool
	// warnOnUnboundMethods causes the registry to emit warning logs if an RPC method
	// has no HttpRule annotation.
	warnOnUnboundMethods bool

	// proto3OptionalNullable specifies whether Proto3 Optional fields should be marked as x-nullable.
	proto3OptionalNullable bool

	// fileOptions is a mapping of file name to additional OpenAPI file options
	fileOptions map[string]*options.Swagger

	// methodOptions is a mapping of fully-qualified method name to additional OpenAPI method options
	methodOptions map[string]*options.Operation

	// messageOptions is a mapping of fully-qualified message name to additional OpenAPI message options
	messageOptions map[string]*options.Schema

	//serviceOptions is a mapping of fully-qualified service name to additional OpenAPI service options
	serviceOptions map[string]*options.Tag

	// fieldOptions is a mapping of the fully-qualified name of the parent message concat
	// field name and a period to additional OpenAPI field options
	fieldOptions map[string]*options.JSONSchema

	// generateUnboundMethods causes the registry to generate proxy methods even for
	// RPC methods that have no HttpRule annotation.
	generateUnboundMethods bool

	// omitPackageDoc, if false, causes a package comment to be included in the generated code.
	omitPackageDoc bool

	// recursiveDepth sets the maximum depth of a field parameter
	recursiveDepth int

	// annotationMap is used to check for duplicate HTTP annotations
	annotationMap map[annotationIdentifier]struct{}

	// disableServiceTags disables the generation of service tags.
	// This is useful if you do not want to expose the names of your backend grpc services.
	disableServiceTags bool

	// disableDefaultResponses disables the generation of default responses.
	// Useful if you have to support custom response codes that are not 200.
	disableDefaultResponses bool

	// useAllOfForRefs, if set, will use allOf as container for $ref to preserve same-level
	// properties
	useAllOfForRefs bool

	// omitArrayItemTypeWhenRefSibling, if set, will omit 'type: object' in array items when $ref is present
	// to avoid no-$ref-siblings violations in OpenAPI v2
	omitArrayItemTypeWhenRefSibling bool

	// allowPatchFeature determines whether to use PATCH feature involving update masks (using google.protobuf.FieldMask).
	allowPatchFeature bool

	// preserveRPCOrder, if true, will ensure the order of paths emitted in openapi swagger files mirror
	// the order of RPC methods found in proto files. If false, emitted paths will be ordered alphabetically.
	preserveRPCOrder bool

	// enableRpcDeprecation whether to process grpc method's deprecated option
	enableRpcDeprecation bool

	// enableFieldDeprecation whether to process proto field's deprecated option
	enableFieldDeprecation bool

	// expandSlashedPathPatterns, if true, for a path parameter carrying a sub-path, described via parameter pattern (i.e.
	// the pattern contains forward slashes), this will expand the _pattern_ into the URI and will _replace_ the parameter
	// with new path parameters inferred from patterns wildcards.
	//
	// Example: a Google AIP style path "/v1/{name=projects/*/locations/*}/datasets/{dataset}" with a "name" parameter
	// containing sub-path will generate "/v1/projects/{project}/locations/{location}/datasets/{dataset}" path in OpenAPI.
	// Note that the original "name" parameter is replaced with "project" and "location" parameters.
	//
	// This leads to more compliant and readable OpenAPI suitable for documentation, but may complicate client
	// implementation if you want to pass the original "name" parameter.
	expandSlashedPathPatterns bool

	// generateXGoType is a global generator option for generating x-go-type annotations
	generateXGoType bool
}

type repeatedFieldSeparator struct {
	name string
	sep  rune
}

type annotationIdentifier struct {
	method       string
	pathTemplate string
	service      *Service
}

// NewRegistry returns a new Registry.
func NewRegistry() *Registry {
	return &Registry{
		msgs:                           make(map[string]*Message),
		enums:                          make(map[string]*Enum),
		meths:                          make(map[string]*Method),
		files:                          make(map[string]*File),
		pkgMap:                         make(map[string]string),
		pkgAliases:                     make(map[string]string),
		externalHTTPRules:              make(map[string][]*annotations.HttpRule),
		openAPINamingStrategy:          "legacy",
		visibilityRestrictionSelectors: make(map[string]bool),
		repeatedPathParamSeparator: repeatedFieldSeparator{
			name: "csv",
			sep:  ',',
		},
		fileOptions:    make(map[string]*options.Swagger),
		methodOptions:  make(map[string]*options.Operation),
		messageOptions: make(map[string]*options.Schema),
		serviceOptions: make(map[string]*options.Tag),
		fieldOptions:   make(map[string]*options.JSONSchema),
		annotationMap:  make(map[annotationIdentifier]struct{}),
		recursiveDepth: 1000,
	}
}

// Load loads definitions of services, methods, messages, enumerations and fields from "req".
func (r *Registry) Load(req *pluginpb.CodeGeneratorRequest) error {
	gen, err := protogen.Options{}.New(req)
	if err != nil {
		return err
	}
	// Note: keep in mind that this might be not enough because
	// protogen.Plugin is used only to load files here.
	// The support for features must be set on the pluginpb.CodeGeneratorResponse.
	codegenerator.SetSupportedFeaturesOnPluginGen(gen)
	return r.load(gen)
}

func (r *Registry) LoadFromPlugin(gen *protogen.Plugin) error {
	return r.load(gen)
}

func (r *Registry) load(gen *protogen.Plugin) error {
	filePaths := make([]string, 0, len(gen.FilesByPath))
	for filePath := range gen.FilesByPath {
		filePaths = append(filePaths, filePath)
	}
	sort.Strings(filePaths)

	for _, filePath := range filePaths {
		r.loadFile(filePath, gen.FilesByPath[filePath])
	}

	for _, filePath := range filePaths {
		if !gen.FilesByPath[filePath].Generate {
			continue
		}
		file := r.files[filePath]
		if err := r.loadServices(file); err != nil {
			return err
		}
	}

	return nil
}

// loadFile loads messages, enumerations and fields from "file".
// It does not load services and methods in "file".  You need to call
// loadServices after loadFiles is called for all files to load services and methods.
func (r *Registry) loadFile(filePath string, file *protogen.File) {
	pkg := GoPackage{
		Path: string(file.GoImportPath),
		Name: string(file.GoPackageName),
	}
	if r.standalone {
		pkg.Alias = "ext" + cases.Title(language.AmericanEnglish).String(pkg.Name)
	}

	if err := r.ReserveGoPackageAlias(pkg.Name, pkg.Path); err != nil {
		for i := 0; ; i++ {
			alias := fmt.Sprintf("%s_%d", pkg.Name, i)
			if err := r.ReserveGoPackageAlias(alias, pkg.Path); err == nil {
				pkg.Alias = alias
				break
			}
		}
	}
	f := &File{
		FileDescriptorProto:     file.Proto,
		GoPkg:                   pkg,
		GeneratedFilenamePrefix: file.GeneratedFilenamePrefix,
	}

	r.files[filePath] = f
	r.registerMsg(f, nil, file.Proto.MessageType)
	r.registerEnum(f, nil, file.Proto.EnumType)
}

func (r *Registry) registerMsg(file *File, outerPath []string, msgs []*descriptorpb.DescriptorProto) {
	for i, md := range msgs {
		m := &Message{
			File:              file,
			Outers:            outerPath,
			DescriptorProto:   md,
			Index:             i,
			ForcePrefixedName: r.standalone,
		}
		for _, fd := range md.GetField() {
			m.Fields = append(m.Fields, &Field{
				Message:              m,
				FieldDescriptorProto: fd,
				ForcePrefixedName:    r.standalone,
			})
		}
		file.Messages = append(file.Messages, m)
		r.msgs[m.FQMN()] = m
		if grpclog.V(1) {
			grpclog.Infof("Register name: %s", m.FQMN())
		}

		var outers []string
		outers = append(outers, outerPath...)
		outers = append(outers, m.GetName())
		r.registerMsg(file, outers, m.GetNestedType())
		r.registerEnum(file, outers, m.GetEnumType())
	}
}

func (r *Registry) registerEnum(file *File, outerPath []string, enums []*descriptorpb.EnumDescriptorProto) {
	for i, ed := range enums {
		e := &Enum{
			File:                file,
			Outers:              outerPath,
			EnumDescriptorProto: ed,
			Index:               i,
			ForcePrefixedName:   r.standalone,
		}
		file.Enums = append(file.Enums, e)
		r.enums[e.FQEN()] = e
		if grpclog.V(1) {
			grpclog.Infof("Register enum name: %s", e.FQEN())
		}
	}
}

// LookupMsg looks up a message type by "name".
// It tries to resolve "name" from "location" if "name" is a relative message name.
func (r *Registry) LookupMsg(location, name string) (*Message, error) {
	if grpclog.V(1) {
		grpclog.Infof("Lookup %s from %s", name, location)
	}
	if strings.HasPrefix(name, ".") {
		m, ok := r.msgs[name]
		if !ok {
			return nil, fmt.Errorf("no message found: %s", name)
		}
		return m, nil
	}

	if !strings.HasPrefix(location, ".") {
		location = fmt.Sprintf(".%s", location)
	}
	components := strings.Split(location, ".")
	for len(components) > 0 {
		fqmn := strings.Join(append(components, name), ".")
		if m, ok := r.msgs[fqmn]; ok {
			return m, nil
		}
		components = components[:len(components)-1]
	}
	return nil, fmt.Errorf("no message found: %s", name)
}

// LookupEnum looks up an enum type by "name".
// It tries to resolve "name" from "location" if "name" is a relative enum name.
func (r *Registry) LookupEnum(location, name string) (*Enum, error) {
	if grpclog.V(1) {
		grpclog.Infof("Lookup enum %s from %s", name, location)
	}
	if strings.HasPrefix(name, ".") {
		e, ok := r.enums[name]
		if !ok {
			return nil, fmt.Errorf("no enum found: %s", name)
		}
		return e, nil
	}

	if !strings.HasPrefix(location, ".") {
		location = fmt.Sprintf(".%s", location)
	}
	components := strings.Split(location, ".")
	for len(components) > 0 {
		fqen := strings.Join(append(components, name), ".")
		if e, ok := r.enums[fqen]; ok {
			return e, nil
		}
		components = components[:len(components)-1]
	}
	return nil, fmt.Errorf("no enum found: %s", name)
}

// LookupFile looks up a file by name.
func (r *Registry) LookupFile(name string) (*File, error) {
	f, ok := r.files[name]
	if !ok {
		return nil, fmt.Errorf("no such file given: %s", name)
	}
	return f, nil
}

func (r *Registry) GetUseProto3FieldSemantics() bool {
	return r.useProto3FieldSemantics
}

func (r *Registry) SetUseProto3FieldSemantics(useProto3FieldSemantics bool) {
	r.useProto3FieldSemantics = useProto3FieldSemantics
}

// LookupExternalHTTPRules looks up external http rules by fully qualified service method name
func (r *Registry) LookupExternalHTTPRules(qualifiedMethodName string) []*annotations.HttpRule {
	return r.externalHTTPRules[qualifiedMethodName]
}

// AddExternalHTTPRule adds an external http rule for the given fully qualified service method name
func (r *Registry) AddExternalHTTPRule(qualifiedMethodName string, rule *annotations.HttpRule) {
	r.externalHTTPRules[qualifiedMethodName] = append(r.externalHTTPRules[qualifiedMethodName], rule)
}

// UnboundExternalHTTPRules returns the list of External HTTPRules
// which does not have a matching method in the registry
func (r *Registry) UnboundExternalHTTPRules() []string {
	allServiceMethods := make(map[string]struct{})
	for _, f := range r.files {
		for _, s := range f.GetService() {
			svc := &Service{File: f, ServiceDescriptorProto: s}
			for _, m := range s.GetMethod() {
				method := &Method{Service: svc, MethodDescriptorProto: m}
				allServiceMethods[method.FQMN()] = struct{}{}
			}
		}
	}

	var missingMethods []string
	for httpRuleMethod := range r.externalHTTPRules {
		if _, ok := allServiceMethods[httpRuleMethod]; !ok {
			missingMethods = append(missingMethods, httpRuleMethod)
		}
	}
	return missingMethods
}

// AddPkgMap adds a mapping from a .proto file to proto package name.
func (r *Registry) AddPkgMap(file, protoPkg string) {
	r.pkgMap[file] = protoPkg
}

// SetPrefix registers the prefix to be added to go package paths generated from proto package names.
func (r *Registry) SetPrefix(prefix string) {
	r.prefix = prefix
}

// SetStandalone registers standalone flag to control package prefix
func (r *Registry) SetStandalone(standalone bool) {
	r.standalone = standalone
}

// SetRecursiveDepth records the max recursion count
func (r *Registry) SetRecursiveDepth(count int) {
	r.recursiveDepth = count
}

// GetRecursiveDepth returns the max recursion count
func (r *Registry) GetRecursiveDepth() int {
	return r.recursiveDepth
}

// ReserveGoPackageAlias reserves the unique alias of go package.
// If succeeded, the alias will be never used for other packages in generated go files.
// If failed, the alias is already taken by another package, so you need to use another
// alias for the package in your go files.
func (r *Registry) ReserveGoPackageAlias(alias, pkgpath string) error {
	if taken, ok := r.pkgAliases[alias]; ok {
		if taken == pkgpath {
			return nil
		}
		return fmt.Errorf("package name %s is already taken. Use another alias", alias)
	}
	r.pkgAliases[alias] = pkgpath
	return nil
}

// GetAllFQMNs returns a list of all FQMNs
func (r *Registry) GetAllFQMNs() []string {
	keys := make([]string, 0, len(r.msgs))
	for k := range r.msgs {
		keys = append(keys, k)
	}
	return keys
}

// GetAllFQENs returns a list of all FQENs
func (r *Registry) GetAllFQENs() []string {
	keys := make([]string, 0, len(r.enums))
	for k := range r.enums {
		keys = append(keys, k)
	}
	return keys
}

func (r *Registry) GetAllFQMethNs() []string {
	keys := make([]string, 0, len(r.meths))
	for k := range r.meths {
		keys = append(keys, k)
	}
	return keys
}

// SetAllowDeleteBody controls whether http delete methods may have a
// body or fail loading if encountered.
func (r *Registry) SetAllowDeleteBody(allow bool) {
	r.allowDeleteBody = allow
}

// SetAllowMerge controls whether generation one OpenAPI file out of multiple protos
func (r *Registry) SetAllowMerge(allow bool) {
	r.allowMerge = allow
}

// IsAllowMerge whether generation one OpenAPI file out of multiple protos
func (r *Registry) IsAllowMerge() bool {
	return r.allowMerge
}

// SetMergeFileName controls the target OpenAPI file name out of multiple protos
func (r *Registry) SetMergeFileName(mergeFileName string) {
	r.mergeFileName = mergeFileName
}

// SetIncludePackageInTags controls whether the package name defined in the `package` directive
// in the proto file can be prepended to the gRPC service name in the `Tags` field of every operation.
func (r *Registry) SetIncludePackageInTags(allow bool) {
	r.includePackageInTags = allow
}

// IsIncludePackageInTags checks whether the package name defined in the `package` directive
// in the proto file can be prepended to the gRPC service name in the `Tags` field of every operation.
func (r *Registry) IsIncludePackageInTags() bool {
	return r.includePackageInTags
}

// GetRepeatedPathParamSeparator returns a rune specifying how
// path parameter repeated fields are separated.
func (r *Registry) GetRepeatedPathParamSeparator() rune {
	return r.repeatedPathParamSeparator.sep
}

// GetRepeatedPathParamSeparatorName returns the name path parameter repeated
// fields repeatedFieldSeparator. I.e. 'csv', 'pipe', 'ssv' or 'tsv'
func (r *Registry) GetRepeatedPathParamSeparatorName() string {
	return r.repeatedPathParamSeparator.name
}

// SetRepeatedPathParamSeparator sets how path parameter repeated fields are
// separated. Allowed names are 'csv', 'pipe', 'ssv' and 'tsv'.
func (r *Registry) SetRepeatedPathParamSeparator(name string) error {
	var sep rune
	switch name {
	case "csv":
		sep = ','
	case "pipes":
		sep = '|'
	case "ssv":
		sep = ' '
	case "tsv":
		sep = '\t'
	default:
		return fmt.Errorf("unknown repeated path parameter separator: %s", name)
	}
	r.repeatedPathParamSeparator = repeatedFieldSeparator{
		name: name,
		sep:  sep,
	}
	return nil
}

// SetUseJSONNamesForFields sets useJSONNamesForFields
func (r *Registry) SetUseJSONNamesForFields(use bool) {
	r.useJSONNamesForFields = use
}

// GetUseJSONNamesForFields returns useJSONNamesForFields
func (r *Registry) GetUseJSONNamesForFields() bool {
	return r.useJSONNamesForFields
}

// SetUseFQNForOpenAPIName sets useFQNForOpenAPIName
// Deprecated: use SetOpenAPINamingStrategy instead.
func (r *Registry) SetUseFQNForOpenAPIName(use bool) {
	r.openAPINamingStrategy = "fqn"
}

// GetUseFQNForOpenAPIName returns useFQNForOpenAPIName
// Deprecated: Use GetOpenAPINamingStrategy().
func (r *Registry) GetUseFQNForOpenAPIName() bool {
	return r.openAPINamingStrategy == "fqn"
}

// GetMergeFileName return the target merge OpenAPI file name
func (r *Registry) GetMergeFileName() string {
	return r.mergeFileName
}

// SetOpenAPINamingStrategy sets the naming strategy to be used.
func (r *Registry) SetOpenAPINamingStrategy(strategy string) {
	r.openAPINamingStrategy = strategy
}

// GetOpenAPINamingStrategy retrieves the naming strategy that is in use.
func (r *Registry) GetOpenAPINamingStrategy() string {
	return r.openAPINamingStrategy
}

// SetUseGoTemplate sets useGoTemplate
func (r *Registry) SetUseGoTemplate(use bool) {
	r.useGoTemplate = use
}

// GetUseGoTemplate returns useGoTemplate
func (r *Registry) GetUseGoTemplate() bool {
	return r.useGoTemplate
}

func (r *Registry) SetGoTemplateArgs(kvs []string) {
	r.goTemplateArgs = make(map[string]string)
	for _, kv := range kvs {
		if key, value, found := strings.Cut(kv, "="); found {
			r.goTemplateArgs[key] = value
		}
	}
}

func (r *Registry) GetGoTemplateArgs() map[string]string {
	return r.goTemplateArgs
}

// SetIgnoreComments sets ignoreComments
func (r *Registry) SetIgnoreComments(ignore bool) {
	r.ignoreComments = ignore
}

// GetIgnoreComments returns ignoreComments
func (r *Registry) GetIgnoreComments() bool {
	return r.ignoreComments
}

// SetRemoveInternalComments sets removeInternalComments
func (r *Registry) SetRemoveInternalComments(remove bool) {
	r.removeInternalComments = remove
}

// GetRemoveInternalComments returns removeInternalComments
func (r *Registry) GetRemoveInternalComments() bool {
	return r.removeInternalComments
}

// SetEnumsAsInts set enumsAsInts
func (r *Registry) SetEnumsAsInts(enumsAsInts bool) {
	r.enumsAsInts = enumsAsInts
}

// GetEnumsAsInts returns enumsAsInts
func (r *Registry) GetEnumsAsInts() bool {
	return r.enumsAsInts
}

// SetOmitEnumDefaultValue sets omitEnumDefaultValue
func (r *Registry) SetOmitEnumDefaultValue(omit bool) {
	r.omitEnumDefaultValue = omit
}

// GetOmitEnumDefaultValue returns omitEnumDefaultValue
func (r *Registry) GetOmitEnumDefaultValue() bool {
	return r.omitEnumDefaultValue
}

// SetVisibilityRestrictionSelectors sets the visibility restriction selectors.
func (r *Registry) SetVisibilityRestrictionSelectors(selectors []string) {
	r.visibilityRestrictionSelectors = make(map[string]bool)
	for _, selector := range selectors {
		r.visibilityRestrictionSelectors[strings.TrimSpace(selector)] = true
	}
}

// GetVisibilityRestrictionSelectors retrieves the visibility restriction selectors.
func (r *Registry) GetVisibilityRestrictionSelectors() map[string]bool {
	return r.visibilityRestrictionSelectors
}

// SetDisableDefaultErrors sets disableDefaultErrors
func (r *Registry) SetDisableDefaultErrors(use bool) {
	r.disableDefaultErrors = use
}

// GetDisableDefaultErrors returns disableDefaultErrors
func (r *Registry) GetDisableDefaultErrors() bool {
	return r.disableDefaultErrors
}

// SetSimpleOperationIDs sets simpleOperationIDs
func (r *Registry) SetSimpleOperationIDs(use bool) {
	r.simpleOperationIDs = use
}

// GetSimpleOperationIDs returns simpleOperationIDs
func (r *Registry) GetSimpleOperationIDs() bool {
	return r.simpleOperationIDs
}

// SetWarnOnUnboundMethods sets warnOnUnboundMethods
func (r *Registry) SetWarnOnUnboundMethods(warn bool) {
	r.warnOnUnboundMethods = warn
}

// SetGenerateUnboundMethods sets generateUnboundMethods
func (r *Registry) SetGenerateUnboundMethods(generate bool) {
	r.generateUnboundMethods = generate
}

// SetOmitPackageDoc controls whether the generated code contains a package comment (if set to false, it will contain one)
func (r *Registry) SetOmitPackageDoc(omit bool) {
	r.omitPackageDoc = omit
}

// GetOmitPackageDoc returns whether a package comment will be omitted from the generated code
func (r *Registry) GetOmitPackageDoc() bool {
	return r.omitPackageDoc
}

// SetProto3OptionalNullable set proto3OptionalNullable
func (r *Registry) SetProto3OptionalNullable(proto3OptionalNullable bool) {
	r.proto3OptionalNullable = proto3OptionalNullable
}

// GetProto3OptionalNullable returns proto3OptionalNullable
func (r *Registry) GetProto3OptionalNullable() bool {
	return r.proto3OptionalNullable
}

// RegisterOpenAPIOptions registers OpenAPI options
func (r *Registry) RegisterOpenAPIOptions(opts *openapiconfig.OpenAPIOptions) error {
	if opts == nil {
		return nil
	}

	for _, opt := range opts.File {
		if _, ok := r.files[opt.File]; !ok {
			return fmt.Errorf("no file %s found", opt.File)
		}
		r.fileOptions[opt.File] = opt.Option
	}

	// build map of all registered methods
	methods := make(map[string]struct{})
	services := make(map[string]struct{})
	for _, f := range r.files {
		for _, s := range f.Services {
			services[s.FQSN()] = struct{}{}
			for _, m := range s.Methods {
				methods[m.FQMN()] = struct{}{}
			}
		}
	}

	for _, opt := range opts.Method {
		qualifiedMethod := "." + opt.Method
		if _, ok := methods[qualifiedMethod]; !ok {
			return fmt.Errorf("no method %s found", opt.Method)
		}
		r.methodOptions[qualifiedMethod] = opt.Option
	}

	for _, opt := range opts.Message {
		qualifiedMessage := "." + opt.Message
		if _, ok := r.msgs[qualifiedMessage]; !ok {
			return fmt.Errorf("no message %s found", opt.Message)
		}
		r.messageOptions[qualifiedMessage] = opt.Option
	}

	for _, opt := range opts.Service {
		qualifiedService := "." + opt.Service
		if _, ok := services[qualifiedService]; !ok {
			return fmt.Errorf("no service %s found", opt.Service)
		}
		r.serviceOptions[qualifiedService] = opt.Option
	}

	// build map of all registered fields
	fields := make(map[string]struct{})
	for _, m := range r.msgs {
		for _, f := range m.Fields {
			fields[f.FQFN()] = struct{}{}
		}
	}
	for _, opt := range opts.Field {
		qualifiedField := "." + opt.Field
		if _, ok := fields[qualifiedField]; !ok {
			return fmt.Errorf("no field %s found", opt.Field)
		}
		r.fieldOptions[qualifiedField] = opt.Option
	}
	return nil
}

// GetOpenAPIFileOption returns a registered OpenAPI option for a file
func (r *Registry) GetOpenAPIFileOption(file string) (*options.Swagger, bool) {
	opt, ok := r.fileOptions[file]
	return opt, ok
}

// GetOpenAPIMethodOption returns a registered OpenAPI option for a method
func (r *Registry) GetOpenAPIMethodOption(qualifiedMethod string) (*options.Operation, bool) {
	opt, ok := r.methodOptions[qualifiedMethod]
	return opt, ok
}

// GetOpenAPIMessageOption returns a registered OpenAPI option for a message
func (r *Registry) GetOpenAPIMessageOption(qualifiedMessage string) (*options.Schema, bool) {
	opt, ok := r.messageOptions[qualifiedMessage]
	return opt, ok
}

// GetOpenAPIServiceOption returns a registered OpenAPI option for a service
func (r *Registry) GetOpenAPIServiceOption(qualifiedService string) (*options.Tag, bool) {
	opt, ok := r.serviceOptions[qualifiedService]
	return opt, ok
}

// GetOpenAPIFieldOption returns a registered OpenAPI option for a field
func (r *Registry) GetOpenAPIFieldOption(qualifiedField string) (*options.JSONSchema, bool) {
	opt, ok := r.fieldOptions[qualifiedField]
	return opt, ok
}

func (r *Registry) FieldName(f *Field) string {
	if r.useJSONNamesForFields {
		return f.GetJsonName()
	}
	return f.GetName()
}

func (r *Registry) CheckDuplicateAnnotation(httpMethod string, httpTemplate string, svc *Service) error {
	a := annotationIdentifier{method: httpMethod, pathTemplate: httpTemplate, service: svc}
	if _, ok := r.annotationMap[a]; ok {
		return fmt.Errorf("duplicate annotation: method=%s, template=%s", httpMethod, httpTemplate)
	}
	r.annotationMap[a] = struct{}{}
	return nil
}

// SetDisableServiceTags sets disableServiceTags
func (r *Registry) SetDisableServiceTags(use bool) {
	r.disableServiceTags = use
}

// GetDisableServiceTags returns disableServiceTags
func (r *Registry) GetDisableServiceTags() bool {
	return r.disableServiceTags
}

// SetDisableDefaultResponses sets disableDefaultResponses
func (r *Registry) SetDisableDefaultResponses(use bool) {
	r.disableDefaultResponses = use
}

// GetDisableDefaultResponses returns disableDefaultResponses
func (r *Registry) GetDisableDefaultResponses() bool {
	return r.disableDefaultResponses
}

// SetUseAllOfForRefs sets useAllOfForRefs
func (r *Registry) SetUseAllOfForRefs(use bool) {
	r.useAllOfForRefs = use
}

// GetUseAllOfForRefs returns useAllOfForRefs
func (r *Registry) GetUseAllOfForRefs() bool {
	return r.useAllOfForRefs
}

// SetOmitArrayItemTypeWhenRefSibling sets omitArrayItemTypeWhenRefSibling
func (r *Registry) SetOmitArrayItemTypeWhenRefSibling(omit bool) {
	r.omitArrayItemTypeWhenRefSibling = omit
}

// GetOmitArrayItemTypeWhenRefSibling returns omitArrayItemTypeWhenRefSibling
func (r *Registry) GetOmitArrayItemTypeWhenRefSibling() bool {
	return r.omitArrayItemTypeWhenRefSibling
}

// SetAllowPatchFeature sets allowPatchFeature
func (r *Registry) SetAllowPatchFeature(allow bool) {
	r.allowPatchFeature = allow
}

// GetAllowPatchFeature returns allowPatchFeature
func (r *Registry) GetAllowPatchFeature() bool {
	return r.allowPatchFeature
}

// SetPreserveRPCOrder sets preserveRPCOrder
func (r *Registry) SetPreserveRPCOrder(preserve bool) {
	r.preserveRPCOrder = preserve
}

// IsPreserveRPCOrder returns preserveRPCOrder
func (r *Registry) IsPreserveRPCOrder() bool {
	return r.preserveRPCOrder
}

// SetEnableRpcDeprecation sets enableRpcDeprecation
func (r *Registry) SetEnableRpcDeprecation(enable bool) {
	r.enableRpcDeprecation = enable
}

// GetEnableRpcDeprecation returns enableRpcDeprecation
func (r *Registry) GetEnableRpcDeprecation() bool {
	return r.enableRpcDeprecation
}

// SetEnableFieldDeprecation sets enableFieldDeprecation
func (r *Registry) SetEnableFieldDeprecation(enable bool) {
	r.enableFieldDeprecation = enable
}

// GetEnableFieldDeprecation returns enableFieldDeprecation
func (r *Registry) GetEnableFieldDeprecation() bool {
	return r.enableFieldDeprecation
}

func (r *Registry) SetExpandSlashedPathPatterns(expandSlashedPathPatterns bool) {
	r.expandSlashedPathPatterns = expandSlashedPathPatterns
}

func (r *Registry) GetExpandSlashedPathPatterns() bool {
	return r.expandSlashedPathPatterns
}

func (r *Registry) SetGenerateXGoType(generateXGoType bool) {
	r.generateXGoType = generateXGoType
}

func (r *Registry) GetGenerateXGoType() bool {
	return r.generateXGoType
}
