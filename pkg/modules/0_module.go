package modules

import (
	"fmt"
	"strings"

	pgs "github.com/lyft/protoc-gen-star/v2"
	"github.com/pubg/protoc-gen-jsonschema/pkg/jsonschema"
	"github.com/pubg/protoc-gen-jsonschema/pkg/meta"
	"github.com/pubg/protoc-gen-jsonschema/pkg/proto"
	"google.golang.org/protobuf/encoding/protojson"
)

type Module struct {
	*pgs.ModuleBase
	pluginOptions *proto.PluginOptions
	metaCollector *meta.Collector
	metaConfig    *metaConfig
}

func NewModule() *Module {
	return &Module{ModuleBase: &pgs.ModuleBase{}}
}

func (m *Module) Name() string {
	return "JsonSchemaModule"
}

func (m *Module) InitContext(c pgs.BuildContext) {
	m.ModuleBase.InitContext(c)
	m.pluginOptions = proto.GetPluginOptions(c.Parameters())
	var err error
	m.metaConfig, err = newMetaConfig(c.Parameters())
	m.CheckErr(err, "invalid meta output configuration")

	m.Debugf("pluginOptions: %v", protojson.MarshalOptions{EmitUnpopulated: true}.Format(m.pluginOptions))
}

func (m *Module) Execute(targets map[string]pgs.File, packages map[string]pgs.Package) []pgs.Artifact {
	m.metaCollector = m.buildMetaCollector(packages)

	// Phase: Frontend IntermediateSchemaGenerate
	visitor := NewVisitor(m, m.pluginOptions, m.metaCollector)
	for _, pkg := range packages {
		m.CheckErr(pgs.Walk(visitor, pkg), fmt.Sprintf("failed to walk package %s", pkg.ProtoName().String()))
	}
	m.Debugf("# of IntermediateSchemas: %d", len(visitor.registry.GetKeys()))

	// Phase: Backend TargetSchemaGenerate
	var optimizer BackendOptimizer = NewOptimizerImpl(m.ModuleBase, m.pluginOptions)
	var generator BackendTargetGenerator = NewMultiDraftGenerator(m.ModuleBase, m.pluginOptions)
	var serializer BackendSerializer = NewSerializerImpl(m.pluginOptions)
	m.Push("BackendPhase")
	visitor.registry.SortSchemas()

	for _, file := range targets {
		artifact := m.BackendPhase(file, visitor.registry, optimizer, generator, serializer)
		if artifact != nil {
			m.AddArtifact(artifact)
		}
	}

	return m.Artifacts()
}

func (m *Module) BackendPhase(file pgs.File, registry *jsonschema.Registry, optimizer BackendOptimizer, generator BackendTargetGenerator, serializer BackendSerializer) pgs.Artifact {
	defer m.Push(file.Name().String()).Pop()
	m.Debugf("FileOptions: %v", protojson.MarshalOptions{EmitUnpopulated: true}.Format(proto.GetFileOptions(file)))

	entrypointMessage := getEntrypointFromFile(file, m.pluginOptions)
	if entrypointMessage == nil {
		m.Logf("Cannot find matched entrypointMessage, Please check FileOptions")
		return nil
	}

	copiedRegistry := jsonschema.DeepCopyRegistry(registry)
	optimizer.Optimize(copiedRegistry, entrypointMessage)
	m.Debugf("# of Schemas After Optimized : %d", len(copiedRegistry.GetKeys()))

	fileOptions := proto.GetFileOptions(file)
	rootSchema := generator.Generate(copiedRegistry, entrypointMessage, fileOptions)
	if rootSchema == nil {
		m.Logf("Cannot generate rootSchema, Please check FileOptions or PluginOptions")
		return nil
	}

	content, err := serializer.Serialize(rootSchema, file)
	m.CheckErr(err, fmt.Sprintf("Failed to serialize file %s", file.Name().String()))
	fileName := serializer.ToFileName(file)
	m.Debugf("GeneratedFileName: %s", fileName)
	if metaArtifact := m.generateMetaArtifact(file); metaArtifact != nil {
		m.AddArtifact(metaArtifact)
	}

	return pgs.GeneratorFile{Name: fileName, Contents: string(content)}
}

func getEntrypointFromFile(file pgs.File, pluginOptions *proto.PluginOptions) pgs.Message {
	entryPointMessage := proto.GetEntrypointMessage(pluginOptions, proto.GetFileOptions(file))
	if entryPointMessage == "" {
		return nil
	}

	for _, message := range file.Messages() {
		if message.Name().String() == entryPointMessage {
			return message
		}
	}
	return nil
}

func collectAllFiles(packages map[string]pgs.Package) []pgs.File {
	if len(packages) == 0 {
		return nil
	}

	seen := map[string]pgs.File{}
	for _, pkg := range packages {
		for _, file := range pkg.Files() {
			seen[file.Name().String()] = file
		}
	}

	files := make([]pgs.File, 0, len(seen))
	for _, file := range seen {
		files = append(files, file)
	}
	return files
}

type metaConfig struct {
	enabled    bool
	fileSuffix string
}

func newMetaConfig(params pgs.Parameters) (*metaConfig, error) {
	enabled, _ := params.BoolDefault("meta_output", false)
	fileSuffix := params.StrDefault("meta_output_file_suffix", ".meta.json")

	if !enabled {
		return &metaConfig{enabled: false, fileSuffix: fileSuffix}, nil
	}

	if fileSuffix == "" {
		return nil, fmt.Errorf("meta_output_file_suffix must not be empty when meta_output=true")
	}
	if !strings.HasSuffix(fileSuffix, ".json") {
		return nil, fmt.Errorf("meta_output_file_suffix must end with .json")
	}

	return &metaConfig{enabled: enabled, fileSuffix: fileSuffix}, nil
}

func (m *Module) generateMetaArtifact(file pgs.File) pgs.Artifact {
	if m.metaConfig == nil || !m.metaConfig.enabled || m.metaCollector == nil {
		return nil
	}

	content, err := m.metaCollector.MarshalFile(file)
	m.CheckErr(err, fmt.Sprintf("failed to serialize meta file for %s", file.Name().String()))

	fileName := file.InputPath().SetExt(m.metaConfig.fileSuffix).String()
	m.Debugf("GeneratedMetaFileName: %s", fileName)
	return pgs.GeneratorFile{Name: fileName, Contents: string(content)}
}

func (m *Module) buildMetaCollector(packages map[string]pgs.Package) *meta.Collector {
	if m.metaConfig == nil || !m.metaConfig.enabled {
		return nil
	}

	files := collectAllFiles(packages)
	resolver, err := meta.BuildResolver(files)
	m.CheckErr(err, "failed to build meta option resolver")
	return meta.NewCollector(resolver)
}
