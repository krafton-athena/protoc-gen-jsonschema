package meta

import (
	"encoding/json"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	stdproto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// OptionsResolver resolves and marshals protobuf options to JSON.
type OptionsResolver struct {
	resolver *protoregistry.Types
}

var protoJSONMarshaler = protojson.MarshalOptions{
	UseProtoNames: true,
}

// NewOptionsResolver creates a new OptionsResolver with the given type resolver.
func NewOptionsResolver(resolver *protoregistry.Types) *OptionsResolver {
	return &OptionsResolver{resolver: resolver}
}

// ResolveOptions converts a protobuf options message to a map for JSON Schema x-options.
func (r *OptionsResolver) ResolveOptions(opts stdproto.Message) map[string]any {
	if opts == nil {
		return nil
	}

	normalized := r.normalizeOptions(opts)
	buf, err := protoJSONMarshaler.Marshal(normalized)
	if err != nil {
		return nil
	}

	var raw map[string]any
	if err := json.Unmarshal(buf, &raw); err != nil {
		return nil
	}

	if len(raw) == 0 {
		return nil
	}

	// Simplify extension field names: "[vita.desc]" -> "desc"
	result := make(map[string]any, len(raw))
	for key, value := range raw {
		result[simplifyOptionKey(key)] = value
	}
	return result
}

// simplifyOptionKey converts "[package.name]" to "name"
func simplifyOptionKey(key string) string {
	// Extension fields are formatted as "[package.name]"
	if strings.HasPrefix(key, "[") && strings.HasSuffix(key, "]") {
		inner := key[1 : len(key)-1] // Remove brackets
		if idx := strings.LastIndex(inner, "."); idx != -1 {
			return inner[idx+1:]
		}
		return inner
	}
	return key
}

func (r *OptionsResolver) normalizeOptions(msg stdproto.Message) stdproto.Message {
	if msg == nil || r.resolver == nil {
		return msg
	}

	raw, err := stdproto.Marshal(msg)
	if err != nil || len(raw) == 0 {
		return msg
	}

	dst := msg.ProtoReflect().New().Interface()
	unmarshaler := stdproto.UnmarshalOptions{Resolver: r.resolver}
	if err := unmarshaler.Unmarshal(raw, dst); err != nil {
		return msg
	}
	return dst
}
