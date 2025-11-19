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

	registry      *jsonschema.Registry
	pluginOptions *proto.PluginOptions
	metaCollector *meta.Collector
}

var _ pgs.Visitor = (*FrontendVisitor)(nil)

func NewVisitor(debugger pgs.DebuggerCommon, pluginOptions *proto.PluginOptions, metaCollector *meta.Collector) *FrontendVisitor {
	v := &FrontendVisitor{
		debugger:      debugger,
		registry:      jsonschema.NewRegistry(),
		pluginOptions: pluginOptions,
		metaCollector: metaCollector,
	}
	v.Visitor = pgs.PassThroughVisitor(v)
	return v
}

func (v *FrontendVisitor) VisitFile(file pgs.File) (pgs.Visitor, error) {
	if v.metaCollector != nil {
		v.metaCollector.SetFileOptions(file, file.Descriptor().GetOptions())
	}
	return v, nil
}

func (v *FrontendVisitor) VisitMessage(message pgs.Message) (pgs.Visitor, error) {
	mo := proto.GetMessageOptions(message)
	if v.metaCollector != nil {
		v.metaCollector.SetMessageOptions(message, message.Descriptor().GetOptions())
	}
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
	v.registry.AddSchema(message.FullyQualifiedName(), schema)
	return v, nil
}

func (v *FrontendVisitor) VisitField(field pgs.Field) (pgs.Visitor, error) {
	fo := proto.GetFieldOptions(field)
	if v.metaCollector != nil {
		v.metaCollector.SetFieldOptions(field, field.Descriptor().GetOptions())
	}
	if fo.GetVisibilityLevel() < v.pluginOptions.GetVisibilityLevel() {
		return nil, nil
	}

	// if field is well-known type
	if isWellKnownField(field) {
		schema := buildFromWellKnownField(field, fo)
		v.registry.AddSchema(field.FullyQualifiedName(), schema)
		return v, nil
	}

	// if field is message or map type
	fieldType := field.Type()
	if fieldType.IsMap() {
		schema := buildFromMapField(v.pluginOptions, field, fo)
		v.registry.AddSchema(field.FullyQualifiedName(), schema)
		return v, nil
	} else if fieldType.ProtoType() == pgs.MessageT {
		schema := buildFromMessageField(field, fo)
		v.registry.AddSchema(field.FullyQualifiedName(), schema)
		return v, nil
	}

	// if field is scala type
	// scala = boolean, string, number
	if isScalarType(field) {
		schema := buildFromScalaField(v.pluginOptions, field, fo)
		v.registry.AddSchema(field.FullyQualifiedName(), schema)
		return v, nil
	}

	panic("not supported field type")
	return v, nil
}

func (v *FrontendVisitor) VisitEnum(enum pgs.Enum) (pgs.Visitor, error) {
	if v.metaCollector != nil {
		v.metaCollector.SetEnumOptions(enum, enum.Descriptor().GetOptions())
		for _, value := range enum.Values() {
			v.metaCollector.SetEnumValueOptions(value, value.Descriptor().GetOptions())
		}
	}
	schema, err := buildFromEnum(enum)
	if err != nil {
		return nil, err
	}
	v.registry.AddSchema(enum.FullyQualifiedName(), schema)
	return v, nil
}
