package meta

import (
	"encoding/json"

	pgs "github.com/lyft/protoc-gen-star/v2"
	"google.golang.org/protobuf/encoding/protojson"
	stdproto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Collector struct {
	files    map[string]*FileMeta
	resolver *protoregistry.Types
}

type FileMeta struct {
	File             string                     `json:"file"`
	FileOptions      json.RawMessage            `json:"file_options,omitempty"`
	MessageOptions   map[string]json.RawMessage `json:"message_options,omitempty"`
	FieldOptions     map[string]json.RawMessage `json:"field_options,omitempty"`
	EnumOptions      map[string]json.RawMessage `json:"enum_options,omitempty"`
	EnumValueOptions map[string]json.RawMessage `json:"enum_value_options,omitempty"`
}

var protoJSONMarshaler = protojson.MarshalOptions{
	UseProtoNames: true,
}

func NewCollector(resolver *protoregistry.Types) *Collector {
	return &Collector{files: map[string]*FileMeta{}, resolver: resolver}
}

func (c *Collector) ensureFile(file pgs.File) *FileMeta {
	key := fileKey(file)
	if entry, ok := c.files[key]; ok {
		return entry
	}

	entry := &FileMeta{File: file.InputPath().String()}
	c.files[key] = entry
	return entry
}

func (c *Collector) SetFileOptions(file pgs.File, opts *descriptorpb.FileOptions) {
	if entry := c.ensureFile(file); opts != nil {
		entry.FileOptions = c.marshalProto(opts)
	}
}

func (c *Collector) SetMessageOptions(message pgs.Message, opts *descriptorpb.MessageOptions) {
	if opts == nil {
		return
	}
	entry := c.ensureFile(message.File())
	if entry.MessageOptions == nil {
		entry.MessageOptions = map[string]json.RawMessage{}
	}
	entry.MessageOptions[message.FullyQualifiedName()] = c.marshalProto(opts)
}

func (c *Collector) SetFieldOptions(field pgs.Field, opts *descriptorpb.FieldOptions) {
	if opts == nil {
		return
	}
	entry := c.ensureFile(field.File())
	if entry.FieldOptions == nil {
		entry.FieldOptions = map[string]json.RawMessage{}
	}
	entry.FieldOptions[field.FullyQualifiedName()] = c.marshalProto(opts)
}

func (c *Collector) SetEnumOptions(enum pgs.Enum, opts *descriptorpb.EnumOptions) {
	if opts == nil {
		return
	}
	entry := c.ensureFile(enum.File())
	if entry.EnumOptions == nil {
		entry.EnumOptions = map[string]json.RawMessage{}
	}
	entry.EnumOptions[enum.FullyQualifiedName()] = c.marshalProto(opts)
}

func (c *Collector) SetEnumValueOptions(value pgs.EnumValue, opts *descriptorpb.EnumValueOptions) {
	if opts == nil {
		return
	}
	entry := c.ensureFile(value.Enum().File())
	if entry.EnumValueOptions == nil {
		entry.EnumValueOptions = map[string]json.RawMessage{}
	}
	entry.EnumValueOptions[value.FullyQualifiedName()] = c.marshalProto(opts)
}

func (c *Collector) MarshalFile(file pgs.File) ([]byte, error) {
	entry := c.ensureFile(file)
	return json.MarshalIndent(entry, "", "  ")
}

func (c *Collector) marshalProto(msg stdproto.Message) json.RawMessage {
	if msg == nil {
		return nil
	}
	normalized := c.normalizeOptions(msg)
	buf, err := protoJSONMarshaler.Marshal(normalized)
	if err != nil {
		return nil
	}
	return buf
}

func (c *Collector) normalizeOptions(msg stdproto.Message) stdproto.Message {
	if msg == nil || c.resolver == nil {
		return msg
	}

	raw, err := stdproto.Marshal(msg)
	if err != nil || len(raw) == 0 {
		return msg
	}

	dst := msg.ProtoReflect().New().Interface()
	unmarshaler := stdproto.UnmarshalOptions{Resolver: c.resolver}
	if err := unmarshaler.Unmarshal(raw, dst); err != nil {
		return msg
	}
	return dst
}

func fileKey(file pgs.File) string {
	return file.InputPath().String()
}
