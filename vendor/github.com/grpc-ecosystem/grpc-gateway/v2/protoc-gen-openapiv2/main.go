package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/internal/codegenerator"
	"github.com/grpc-ecosystem/grpc-gateway/v2/internal/descriptor"
	"github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/internal/genopenapi"
	"github.com/grpc-ecosystem/grpc-gateway/v2/utilities"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

var (
	importPrefix                    = flag.String("import_prefix", "", "prefix to be added to go package paths for imported proto files")
	file                            = flag.String("file", "-", "where to load data from")
	allowDeleteBody                 = flag.Bool("allow_delete_body", false, "unless set, HTTP DELETE methods may not have a body")
	grpcAPIConfiguration            = flag.String("grpc_api_configuration", "", "path to file which describes the gRPC API Configuration in YAML format")
	allowMerge                      = flag.Bool("allow_merge", false, "if set, generation one OpenAPI file out of multiple protos")
	mergeFileName                   = flag.String("merge_file_name", "apidocs", "target OpenAPI file name prefix after merge")
	useJSONNamesForFields           = flag.Bool("json_names_for_fields", true, "if disabled, the original proto name will be used for generating OpenAPI definitions")
	repeatedPathParamSeparator      = flag.String("repeated_path_param_separator", "csv", "configures how repeated fields should be split. Allowed values are `csv`, `pipes`, `ssv` and `tsv`")
	versionFlag                     = flag.Bool("version", false, "print the current version")
	_                               = flag.Bool("allow_repeated_fields_in_body", true, "allows to use repeated field in `body` and `response_body` field of `google.api.http` annotation option. DEPRECATED: the value is ignored and always behaves as `true`.")
	includePackageInTags            = flag.Bool("include_package_in_tags", false, "if unset, the gRPC service name is added to the `Tags` field of each operation. If set and the `package` directive is shown in the proto file, the package name will be prepended to the service name")
	useFQNForOpenAPIName            = flag.Bool("fqn_for_openapi_name", false, "if set, the object's OpenAPI names will use the fully qualified names from the proto definition (ie my.package.MyMessage.MyInnerMessage). DEPRECATED: prefer `openapi_naming_strategy=fqn`")
	openAPINamingStrategy           = flag.String("openapi_naming_strategy", "", "use the given OpenAPI naming strategy. Allowed values are `legacy`, `fqn`, `simple`, `package`. If unset, either `legacy` or `fqn` are selected, depending on the value of the `fqn_for_openapi_name` flag")
	useGoTemplate                   = flag.Bool("use_go_templates", false, "if set, you can use Go templates in protofile comments")
	goTemplateArgs                  = utilities.StringArrayFlag(flag.CommandLine, "go_template_args", "provide a custom value that can override a key in the Go template. Requires the `use_go_templates` option to be set")
	ignoreComments                  = flag.Bool("ignore_comments", false, "if set, all protofile comments are excluded from output")
	removeInternalComments          = flag.Bool("remove_internal_comments", false, "if set, removes all substrings in comments that start with `(--` and end with `--)` as specified in https://google.aip.dev/192#internal-comments")
	disableDefaultErrors            = flag.Bool("disable_default_errors", false, "if set, disables generation of default errors. This is useful if you have defined custom error handling")
	enumsAsInts                     = flag.Bool("enums_as_ints", false, "whether to render enum values as integers, as opposed to string values")
	simpleOperationIDs              = flag.Bool("simple_operation_ids", false, "whether to remove the service prefix in the operationID generation. Can introduce duplicate operationIDs, use with caution.")
	proto3OptionalNullable          = flag.Bool("proto3_optional_nullable", false, "whether Proto3 Optional fields should be marked as x-nullable")
	openAPIConfiguration            = flag.String("openapi_configuration", "", "path to file which describes the OpenAPI Configuration in YAML format")
	generateUnboundMethods          = flag.Bool("generate_unbound_methods", false, "generate swagger metadata even for RPC methods that have no HttpRule annotation")
	recursiveDepth                  = flag.Int("recursive-depth", 1000, "maximum recursion count allowed for a field type")
	omitEnumDefaultValue            = flag.Bool("omit_enum_default_value", false, "if set, omit default enum value")
	outputFormat                    = flag.String("output_format", string(genopenapi.FormatJSON), fmt.Sprintf("output content format. Allowed values are: `%s`, `%s`", genopenapi.FormatJSON, genopenapi.FormatYAML))
	visibilityRestrictionSelectors  = utilities.StringArrayFlag(flag.CommandLine, "visibility_restriction_selectors", "list of `google.api.VisibilityRule` visibility labels to include in the generated output when a visibility annotation is defined. Repeat this option to supply multiple values. Elements without visibility annotations are unaffected by this setting.")
	disableServiceTags              = flag.Bool("disable_service_tags", false, "if set, disables generation of service tags. This is useful if you do not want to expose the names of your backend grpc services.")
	disableDefaultResponses         = flag.Bool("disable_default_responses", false, "if set, disables generation of default responses. Useful if you have to support custom response codes that are not 200.")
	useAllOfForRefs                 = flag.Bool("use_allof_for_refs", false, "if set, will use allOf as container for $ref to preserve same-level properties.")
	omitArrayItemTypeWhenRefSibling = flag.Bool("omit_array_item_type_when_ref_sibling", false, "if set, will omit 'type: object' in array items when $ref is present to avoid strict no-$ref-siblings rule violations.")
	allowPatchFeature               = flag.Bool("allow_patch_feature", true, "whether to hide update_mask fields in PATCH requests from the generated swagger file.")
	preserveRPCOrder                = flag.Bool("preserve_rpc_order", false, "if true, will ensure the order of paths emitted in openapi swagger files mirror the order of RPC methods found in proto files. If false, emitted paths will be ordered alphabetically.")
	enableRpcDeprecation            = flag.Bool("enable_rpc_deprecation", false, "whether to process grpc method's deprecated option.")
	enableFieldDeprecation          = flag.Bool("enable_field_deprecation", false, "whether to process proto field's deprecated option.")
	expandSlashedPathPatterns       = flag.Bool("expand_slashed_path_patterns", false, "if set, expands path parameters with URI sub-paths into the URI. For example, \"/v1/{name=projects/*}/resource\" becomes \"/v1/projects/{project}/resource\".")
	useProto3FieldSemantics         = flag.Bool("use_proto3_field_semantics", false, "if set, uses proto3 field semantics for the OpenAPI schema. This means that fields are required by default.")
	generateXGoType                 = flag.Bool("generate_x_go_type", false, "if set, generates x-go-type extension using the go_package option from proto files")

	_ = flag.Bool("logtostderr", false, "Legacy glog compatibility. This flag is a no-op, you can safely remove it")
)

// Variables set by goreleaser at build time
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	flag.Parse()

	if *versionFlag {
		if commit == "unknown" {
			buildInfo, ok := debug.ReadBuildInfo()
			if ok {
				version = buildInfo.Main.Version
				for _, setting := range buildInfo.Settings {
					if setting.Key == "vcs.revision" {
						commit = setting.Value
					}
					if setting.Key == "vcs.time" {
						date = setting.Value
					}
				}
			}
		}
		fmt.Printf("Version %v, commit %v, built at %v\n", version, commit, date)
		os.Exit(0)
	}

	reg := descriptor.NewRegistry()
	if grpclog.V(1) {
		grpclog.Info("Processing code generator request")
	}
	f := os.Stdin
	if *file != "-" {
		var err error
		f, err = os.Open(*file)
		if err != nil {
			grpclog.Fatal(err)
		}
	}
	if grpclog.V(1) {
		grpclog.Info("Parsing code generator request")
	}
	req, err := codegenerator.ParseRequest(f)
	if err != nil {
		grpclog.Fatal(err)
	}
	if grpclog.V(1) {
		grpclog.Info("Parsed code generator request")
	}
	pkgMap := make(map[string]string)
	if req.Parameter != nil {
		if err := parseReqParam(req.GetParameter(), flag.CommandLine, pkgMap); err != nil {
			grpclog.Fatalf("Error parsing flags: %v", err)
		}
	}

	reg.SetPrefix(*importPrefix)
	reg.SetAllowDeleteBody(*allowDeleteBody)
	reg.SetAllowMerge(*allowMerge)
	reg.SetMergeFileName(*mergeFileName)
	reg.SetUseJSONNamesForFields(*useJSONNamesForFields)
	reg.SetUseProto3FieldSemantics(*useProto3FieldSemantics)

	flag.Visit(func(f *flag.Flag) {
		if f.Name == "allow_repeated_fields_in_body" {
			grpclog.Warning("The `allow_repeated_fields_in_body` flag is deprecated and will always behave as `true`.")
		}
	})

	reg.SetIncludePackageInTags(*includePackageInTags)

	reg.SetUseFQNForOpenAPIName(*useFQNForOpenAPIName)
	// Set the naming strategy either directly from the flag, or via the value of the legacy fqn_for_openapi_name
	// flag.
	namingStrategy := *openAPINamingStrategy
	if *useFQNForOpenAPIName {
		if namingStrategy != "" {
			grpclog.Fatal("The deprecated `fqn_for_openapi_name` flag must remain unset if `openapi_naming_strategy` is set.")
		}
		grpclog.Warning("The `fqn_for_openapi_name` flag is deprecated. Please use `openapi_naming_strategy=fqn` instead.")
		namingStrategy = "fqn"
	} else if namingStrategy == "" {
		namingStrategy = "legacy"
	}
	if strategyFn := genopenapi.LookupNamingStrategy(namingStrategy); strategyFn == nil {
		emitError(fmt.Errorf("invalid naming strategy %q", namingStrategy))
		return
	}

	if *useGoTemplate && *ignoreComments {
		emitError(fmt.Errorf("`ignore_comments` and `use_go_templates` are mutually exclusive and cannot be enabled at the same time"))
		return
	}
	reg.SetUseGoTemplate(*useGoTemplate)
	reg.SetIgnoreComments(*ignoreComments)
	reg.SetRemoveInternalComments(*removeInternalComments)

	if len(*goTemplateArgs) > 0 && !*useGoTemplate {
		emitError(fmt.Errorf("`go_template_args` requires `use_go_templates` to be enabled"))
		return
	}
	reg.SetGoTemplateArgs(*goTemplateArgs)

	reg.SetOpenAPINamingStrategy(namingStrategy)
	reg.SetEnumsAsInts(*enumsAsInts)
	reg.SetDisableDefaultErrors(*disableDefaultErrors)
	reg.SetSimpleOperationIDs(*simpleOperationIDs)
	reg.SetProto3OptionalNullable(*proto3OptionalNullable)
	reg.SetGenerateUnboundMethods(*generateUnboundMethods)
	reg.SetRecursiveDepth(*recursiveDepth)
	reg.SetOmitEnumDefaultValue(*omitEnumDefaultValue)
	reg.SetVisibilityRestrictionSelectors(*visibilityRestrictionSelectors)
	reg.SetDisableServiceTags(*disableServiceTags)
	reg.SetDisableDefaultResponses(*disableDefaultResponses)
	reg.SetUseAllOfForRefs(*useAllOfForRefs)
	reg.SetOmitArrayItemTypeWhenRefSibling(*omitArrayItemTypeWhenRefSibling)
	reg.SetAllowPatchFeature(*allowPatchFeature)
	reg.SetPreserveRPCOrder(*preserveRPCOrder)
	reg.SetEnableRpcDeprecation(*enableRpcDeprecation)
	reg.SetEnableFieldDeprecation(*enableFieldDeprecation)
	reg.SetExpandSlashedPathPatterns(*expandSlashedPathPatterns)
	reg.SetGenerateXGoType(*generateXGoType)

	if err := reg.SetRepeatedPathParamSeparator(*repeatedPathParamSeparator); err != nil {
		emitError(err)
		return
	}
	for k, v := range pkgMap {
		reg.AddPkgMap(k, v)
	}

	if *grpcAPIConfiguration != "" {
		if err := reg.LoadGrpcAPIServiceFromYAML(*grpcAPIConfiguration); err != nil {
			emitError(err)
			return
		}
	}

	format := genopenapi.Format(*outputFormat)
	if err := format.Validate(); err != nil {
		emitError(err)
		return
	}

	g := genopenapi.New(reg, format)

	if err := genopenapi.AddErrorDefs(reg); err != nil {
		emitError(err)
		return
	}

	if err := reg.Load(req); err != nil {
		emitError(err)
		return
	}

	if *openAPIConfiguration != "" {
		if err := reg.LoadOpenAPIConfigFromYAML(*openAPIConfiguration); err != nil {
			emitError(err)
			return
		}
	}

	targets := make([]*descriptor.File, 0, len(req.FileToGenerate))
	for _, target := range req.FileToGenerate {
		f, err := reg.LookupFile(target)
		if err != nil {
			grpclog.Fatal(err)
		}
		targets = append(targets, f)
	}

	out, err := g.Generate(targets)
	if grpclog.V(1) {
		grpclog.Info("Processed code generator request")
	}
	if err != nil {
		emitError(err)
		return
	}
	emitFiles(out)
}

func emitFiles(out []*descriptor.ResponseFile) {
	files := make([]*pluginpb.CodeGeneratorResponse_File, len(out))
	for idx, item := range out {
		files[idx] = item.CodeGeneratorResponse_File
	}
	resp := &pluginpb.CodeGeneratorResponse{File: files}
	codegenerator.SetSupportedFeaturesOnCodeGeneratorResponse(resp)
	emitResp(resp)
}

func emitError(err error) {
	emitResp(&pluginpb.CodeGeneratorResponse{Error: proto.String(err.Error())})
}

func emitResp(resp *pluginpb.CodeGeneratorResponse) {
	buf, err := proto.Marshal(resp)
	if err != nil {
		grpclog.Fatal(err)
	}
	if _, err := os.Stdout.Write(buf); err != nil {
		grpclog.Fatal(err)
	}
}

// parseReqParam parses a CodeGeneratorRequest parameter and adds the
// extracted values to the given FlagSet and pkgMap. Returns a non-nil
// error if setting a flag failed.
func parseReqParam(param string, f *flag.FlagSet, pkgMap map[string]string) error {
	if param == "" {
		return nil
	}
	for _, p := range strings.Split(param, ",") {
		spec := strings.SplitN(p, "=", 2)
		if len(spec) == 1 {
			switch spec[0] {
			case "allow_delete_body":
				if err := f.Set(spec[0], "true"); err != nil {
					return fmt.Errorf("cannot set flag %s: %w", p, err)
				}
				continue
			case "allow_merge":
				if err := f.Set(spec[0], "true"); err != nil {
					return fmt.Errorf("cannot set flag %s: %w", p, err)
				}
				continue
			case "allow_repeated_fields_in_body":
				if err := f.Set(spec[0], "true"); err != nil {
					return fmt.Errorf("cannot set flag %s: %w", p, err)
				}
				continue
			case "include_package_in_tags":
				if err := f.Set(spec[0], "true"); err != nil {
					return fmt.Errorf("cannot set flag %s: %w", p, err)
				}
				continue
			}
			if err := f.Set(spec[0], ""); err != nil {
				return fmt.Errorf("cannot set flag %s: %w", p, err)
			}
			continue
		}
		name, value := spec[0], spec[1]
		if strings.HasPrefix(name, "M") {
			pkgMap[name[1:]] = value
			continue
		}
		if err := f.Set(name, value); err != nil {
			return fmt.Errorf("cannot set flag %s: %w", p, err)
		}
	}
	return nil
}
