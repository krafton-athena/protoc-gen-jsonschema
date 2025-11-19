package meta

import (
	"slices"
	"strings"

	pgs "github.com/lyft/protoc-gen-star/v2"
	stdproto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func BuildResolver(files []pgs.File) (*protoregistry.Types, error) {
	if len(files) == 0 {
		return nil, nil
	}

	fdset := &descriptorpb.FileDescriptorSet{}
	seen := map[string]struct{}{}
	for _, file := range files {
		name := file.Name().String()
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		fdset.File = append(fdset.File, stdproto.Clone(file.Descriptor()).(*descriptorpb.FileDescriptorProto))
	}

	slices.SortFunc(fdset.File, func(a, b *descriptorpb.FileDescriptorProto) int {
		return strings.Compare(a.GetName(), b.GetName())
	})

	filesRegistry, err := protodesc.NewFiles(fdset)
	if err != nil {
		return nil, err
	}

	types := &protoregistry.Types{}
	filesRegistry.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		registerFileDescriptors(fd, types)
		return true
	})
	return types, nil
}

func registerFileDescriptors(fd protoreflect.FileDescriptor, types *protoregistry.Types) {
	if types == nil {
		return
	}

	xds := fd.Extensions()
	for i := 0; i < xds.Len(); i++ {
		_ = types.RegisterExtension(dynamicpb.NewExtensionType(xds.Get(i)))
	}

	eds := fd.Enums()
	for i := 0; i < eds.Len(); i++ {
		_ = types.RegisterEnum(dynamicpb.NewEnumType(eds.Get(i)))
	}

	mds := fd.Messages()
	for i := 0; i < mds.Len(); i++ {
		registerMessageDescriptors(mds.Get(i), types)
	}
}

func registerMessageDescriptors(md protoreflect.MessageDescriptor, types *protoregistry.Types) {
	_ = types.RegisterMessage(dynamicpb.NewMessageType(md))

	xds := md.Extensions()
	for i := 0; i < xds.Len(); i++ {
		_ = types.RegisterExtension(dynamicpb.NewExtensionType(xds.Get(i)))
	}

	eds := md.Enums()
	for i := 0; i < eds.Len(); i++ {
		_ = types.RegisterEnum(dynamicpb.NewEnumType(eds.Get(i)))
	}

	mds := md.Messages()
	for i := 0; i < mds.Len(); i++ {
		registerMessageDescriptors(mds.Get(i), types)
	}
}
