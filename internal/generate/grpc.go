package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"lion/internal/model"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

// GRPCGenerator gRPC 反射生成器
type GRPCGenerator struct {
	opts *Options
}

// NewGRPCGenerator 创建 gRPC 生成器
func NewGRPCGenerator(opts *Options) (*GRPCGenerator, error) {
	return &GRPCGenerator{opts: opts}, nil
}

// Generate 通过 gRPC 反射生成 Collection
func (g *GRPCGenerator) Generate() (*model.Collection, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, g.opts.Source,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("连接 gRPC 服务失败: %w", err)
	}
	defer conn.Close()

	// 列出所有服务
	services, err := listServices(ctx, conn)
	if err != nil {
		return nil, err
	}

	collection := &model.Collection{
		Name:      g.opts.Name,
		Source:    "grpc",
		SourceURL: g.opts.Source,
	}

	for _, svcName := range services {
		// 跳过 gRPC 内置服务
		if isInternalService(svcName) {
			continue
		}

		svcDesc, err := resolveServiceDescriptor(ctx, conn, svcName)
		if err != nil {
			fmt.Printf("⚠ 解析服务 %s 失败: %v\n", svcName, err)
			continue
		}

		group := &model.Group{
			Name: svcName,
		}

		methods := svcDesc.Methods()
		for i := 0; i < methods.Len(); i++ {
			md := methods.Get(i)
			fullMethod := fmt.Sprintf("%s/%s", svcName, md.Name())

			bodyJSON := defaultJSONForMessage(md.Input())

			api := &model.API{
				ID:          fullMethod,
				Method:      "gRPC",
				Path:        fullMethod,
				BodyJSON:    bodyJSON,
				Description: "",
			}

			group.APIs = append(group.APIs, api)
		}

		if len(group.APIs) > 0 {
			collection.Groups = append(collection.Groups, group)
		}
	}

	return collection, nil
}

// listServices 通过 gRPC Server Reflection 列出所有服务
func listServices(ctx context.Context, conn *grpc.ClientConn) ([]string, error) {
	refClient := reflectionpb.NewServerReflectionClient(conn)
	stream, err := refClient.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("创建反射流失败: %w", err)
	}

	if err := stream.Send(&reflectionpb.ServerReflectionRequest{
		MessageRequest: &reflectionpb.ServerReflectionRequest_ListServices{
			ListServices: "*",
		},
	}); err != nil {
		return nil, fmt.Errorf("发送 ListServices 请求失败: %w", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("接收 ListServices 响应失败: %w", err)
	}

	listResp, ok := resp.MessageResponse.(*reflectionpb.ServerReflectionResponse_ListServicesResponse)
	if !ok {
		if errResp, ok := resp.MessageResponse.(*reflectionpb.ServerReflectionResponse_ErrorResponse); ok {
			return nil, fmt.Errorf("反射错误 [%v]: %s", errResp.ErrorResponse.ErrorCode, errResp.ErrorResponse.ErrorMessage)
		}
		return nil, fmt.Errorf("ListServices 响应类型不匹配")
	}

	var services []string
	for _, svc := range listResp.ListServicesResponse.Service {
		services = append(services, svc.Name)
	}
	return services, nil
}

// isInternalService 判断是否为 gRPC 内置服务
func isInternalService(name string) bool {
	return strings.HasPrefix(name, "grpc.") ||
		strings.HasPrefix(name, "grpc.reflection.") ||
		strings.HasPrefix(name, "grpc.channelz.") ||
		strings.HasPrefix(name, "grpc.health.")
}

// resolveServiceDescriptor 通过反射获取服务的 protoreflect 描述符
func resolveServiceDescriptor(ctx context.Context, conn *grpc.ClientConn, serviceName string) (protoreflect.ServiceDescriptor, error) {
	refClient := reflectionpb.NewServerReflectionClient(conn)
	stream, err := refClient.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("创建反射流失败: %w", err)
	}

	if err := stream.Send(&reflectionpb.ServerReflectionRequest{
		MessageRequest: &reflectionpb.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: serviceName,
		},
	}); err != nil {
		return nil, fmt.Errorf("发送反射请求失败: %w", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("接收反射响应失败: %w", err)
	}

	fileResp, ok := resp.MessageResponse.(*reflectionpb.ServerReflectionResponse_FileDescriptorResponse)
	if !ok {
		if errResp, ok := resp.MessageResponse.(*reflectionpb.ServerReflectionResponse_ErrorResponse); ok {
			return nil, fmt.Errorf("反射错误 [%v]: %s", errResp.ErrorResponse.ErrorCode, errResp.ErrorResponse.ErrorMessage)
		}
		return nil, fmt.Errorf("反射响应类型不匹配")
	}

	// 解析文件描述符
	files := &descriptorpb.FileDescriptorSet{}
	for _, fdBytes := range fileResp.FileDescriptorResponse.FileDescriptorProto {
		fdp := &descriptorpb.FileDescriptorProto{}
		if err := proto.Unmarshal(fdBytes, fdp); err != nil {
			return nil, fmt.Errorf("解析文件描述符失败: %w", err)
		}
		files.File = append(files.File, fdp)
	}

	// 构建文件描述符注册表
	registry, err := protodesc.NewFiles(files)
	if err != nil {
		return nil, fmt.Errorf("构建描述符注册表失败: %w", err)
	}

	// 查找服务
	serviceFullName := protoreflect.FullName(serviceName)
	desc, err := registry.FindDescriptorByName(serviceFullName)
	if err != nil {
		return nil, fmt.Errorf("查找服务 %s 失败: %w", serviceName, err)
	}

	svcDesc, ok := desc.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("%s 不是服务描述", serviceName)
	}

	return svcDesc, nil
}

// defaultJSONForMessage 根据 message descriptor 生成默认 JSON 字符串
func defaultJSONForMessage(msgDesc protoreflect.MessageDescriptor) string {
	m := buildDefaultMap(msgDesc, 0)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}

// buildDefaultMap 递归构建消息的默认值 map
func buildDefaultMap(msgDesc protoreflect.MessageDescriptor, depth int) map[string]interface{} {
	if depth > 3 {
		return map[string]interface{}{}
	}

	result := make(map[string]interface{})
	fields := msgDesc.Fields()

	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)

		// 跳过 oneof 中的非首选字段
		if field.ContainingOneof() != nil && !field.ContainingOneof().IsSynthetic() {
			if field.ContainingOneof().Fields().Get(0) != field {
				continue
			}
		}

		result[string(field.Name())] = defaultJSONForField(field, depth)
	}

	return result
}

// defaultJSONForField 根据字段类型生成默认值
func defaultJSONForField(field protoreflect.FieldDescriptor, depth int) interface{} {
	// repeated 字段 -> 空数组
	if field.IsList() {
		return []interface{}{}
	}

	// map 字段 -> 空对象
	if field.IsMap() {
		return map[string]interface{}{}
	}

	switch field.Kind() {
	case protoreflect.StringKind, protoreflect.BytesKind:
		return ""
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Uint32Kind,
		protoreflect.Fixed32Kind, protoreflect.Sfixed32Kind:
		return 0
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Uint64Kind,
		protoreflect.Fixed64Kind, protoreflect.Sfixed64Kind:
		return 0
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return 0
	case protoreflect.BoolKind:
		return false
	case protoreflect.EnumKind:
		enumDesc := field.Enum()
		if enumDesc != nil && enumDesc.Values().Len() > 0 {
			return string(enumDesc.Values().Get(0).Name())
		}
		return 0
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return buildDefaultMap(field.Message(), depth+1)
	default:
		return nil
	}
}
