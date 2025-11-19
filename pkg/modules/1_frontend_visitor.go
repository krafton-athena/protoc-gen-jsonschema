package modules

import (
	pgs "github.com/lyft/protoc-gen-star/v2"
	"github.com/pubg/protoc-gen-jsonschema/pkg/jsonschema"
	"github.com/pubg/protoc-gen-jsonschema/pkg/meta"
	"github.com/pubg/protoc-gen-jsonschema/pkg/proto"
)

// FrontendVisitor generate intermediate jsonschema from protobuf
type FrontendVisitor struct {
	pgs.Visitor

	debugger pgs.DebuggerCommon

	registry        *jsonschema.Registry
	pluginOptions   *proto.PluginOptions
	optionsResolver *meta.OptionsResolver
}

var _ pgs.Visitor = (*FrontendVisitor)(nil)

func NewVisitor(debugger pgs.DebuggerCommon, pluginOptions *proto.PluginOptions, optionsResolver *meta.OptionsResolver) *FrontendVisitor {
	v := &FrontendVisitor{
		debugger:        debugger,
		registry:        jsonschema.NewRegistry(),
		pluginOptions:   pluginOptions,
		optionsResolver: optionsResolver,
	}
	v.Visitor = pgs.PassThroughVisitor(v)
	return v
}

func (v *FrontendVisitor) VisitFile(file pgs.File) (pgs.Visitor, error) {
	return v, nil
}

func (v *FrontendVisitor) VisitMessage(message pgs.Message) (pgs.Visitor, error) {
	mo := proto.GetMessageOptions(message)
	if mo.GetVisibilityLevel() < v.pluginOptions.GetVisibilityLevel() {
		return nil, nil
	}

	var schema *jsonschema.Schema
	// if message is well-known type
	if isWellKnownMessage(message) {
		schema = buildFromWellKnownMessage(v.pluginOptions, message, mo)
	} else {
		schema = buildFromMessage(v.pluginOptions, message, mo)
	}

	// Add x-options from message options
	if v.optionsResolver != nil {
		if opts := v.optionsResolver.ResolveOptions(message.Descriptor().GetOptions()); opts != nil {
			schema.SetExtrasItem("x-options", opts)
		}
	}

	v.registry.AddSchema(message.FullyQualifiedName(), schema)
	return v, nil
}

func (v *FrontendVisitor) VisitField(field pgs.Field) (pgs.Visitor, error) {
	fo := proto.GetFieldOptions(field)
	if fo.GetVisibilityLevel() < v.pluginOptions.GetVisibilityLevel() {
		return nil, nil
	}

	var schema *jsonschema.Schema

	// if field is well-known type
	if isWellKnownField(field) {
		schema = buildFromWellKnownField(field, fo)
	} else if fieldType := field.Type(); fieldType.IsMap() {
		// if field is map type
		schema = buildFromMapField(v.pluginOptions, field, fo)
	} else if fieldType.ProtoType() == pgs.MessageT {
		// if field is message type
		schema = buildFromMessageField(field, fo)
	} else if isScalarType(field) {
		// if field is scalar type (boolean, string, number)
		schema = buildFromScalaField(v.pluginOptions, field, fo)
	} else {
		panic("not supported field type")
	}

	// Add x-options from field options
	if v.optionsResolver != nil {
		if opts := v.optionsResolver.ResolveOptions(field.Descriptor().GetOptions()); opts != nil {
			schema.SetExtrasItem("x-options", opts)
		}
	}

	v.registry.AddSchema(field.FullyQualifiedName(), schema)
	return v, nil
}

func (v *FrontendVisitor) VisitEnum(enum pgs.Enum) (pgs.Visitor, error) {
	schema, err := buildFromEnum(enum)
	if err != nil {
		return nil, err
	}

	// Add x-options from enum options
	if v.optionsResolver != nil {
		if opts := v.optionsResolver.ResolveOptions(enum.Descriptor().GetOptions()); opts != nil {
			schema.SetExtrasItem("x-options", opts)
		}
		// Add enum value options as x-enum-value-options
		valueOpts := make(map[string]any)
		for _, value := range enum.Values() {
			if valOpts := v.optionsResolver.ResolveOptions(value.Descriptor().GetOptions()); valOpts != nil {
				valueOpts[value.Name().String()] = valOpts
			}
		}
		if len(valueOpts) > 0 {
			schema.SetExtrasItem("x-enum-value-options", valueOpts)
		}
	}

	v.registry.AddSchema(enum.FullyQualifiedName(), schema)
	return v, nil
}
