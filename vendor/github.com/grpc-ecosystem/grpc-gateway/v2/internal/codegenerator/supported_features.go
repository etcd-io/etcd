package codegenerator

import (
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func supportedCodeGeneratorFeatures() uint64 {
	// Enable support for Protobuf Editions
	return uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL | pluginpb.CodeGeneratorResponse_FEATURE_SUPPORTS_EDITIONS)
}

func supportedEditions() (descriptorpb.Edition, descriptorpb.Edition) {
	// Declare support up to edition 2024
	return descriptorpb.Edition_EDITION_2023, descriptorpb.Edition_EDITION_2024
}

// SetSupportedFeaturesOnPluginGen sets supported proto3 features
// on protogen.Plugin.
func SetSupportedFeaturesOnPluginGen(gen *protogen.Plugin) {
	gen.SupportedFeatures = supportedCodeGeneratorFeatures()
	gen.SupportedEditionsMinimum, gen.SupportedEditionsMaximum = supportedEditions()
}

// SetSupportedFeaturesOnCodeGeneratorResponse sets supported proto3 features
// on pluginpb.CodeGeneratorResponse.
func SetSupportedFeaturesOnCodeGeneratorResponse(resp *pluginpb.CodeGeneratorResponse) {
	sf := supportedCodeGeneratorFeatures()
	resp.SupportedFeatures = &sf
	minE, maxE := supportedEditions()
	minEN, maxEN := int32(minE.Number()), int32(maxE.Number())
	resp.MinimumEdition, resp.MaximumEdition = &minEN, &maxEN
}
