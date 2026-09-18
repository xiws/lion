package sender

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"lion/internal/model"
	"lion/internal/script"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

// SendGRPC 发送 gRPC 请求
func (s *Sender) SendGRPC(api *model.API, params map[string]string, overrides map[string]string) error {
	host, err := s.resolveHost()
	if err != nil {
		return err
	}

	timeout := parseTimeout(s.cfg.Global.Timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, host,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("连接 gRPC 服务失败: %w", err)
	}
	defer conn.Close()

	// 构造请求体
	var body interface{}
	if api.BodyJSON != "" {
		if err := json.Unmarshal([]byte(api.BodyJSON), &body); err != nil {
			return fmt.Errorf("解析请求体失败: %w", err)
		}
	}
	if len(overrides) > 0 {
		if bodyMap, ok := body.(map[string]interface{}); ok {
			for k, v := range overrides {
				bodyMap[k] = v
			}
		}
	}

	// 构造脚本上下文
	reqCtx := &model.ScriptContext{
		APIID:   api.ID,
		Method:  "gRPC",
		URL:     fmt.Sprintf("grpc://%s/%s", host, api.Path),
		Headers: make(map[string]string),
		Params:  params,
		Body:    body,
	}

	// 执行前置脚本
	preScript := script.ResolveScript(api, s.cfg.Hooks, true)
	if preScript != "" {
		result, err := s.engine.ExecutePreScript(preScript, "pre_request", reqCtx)
		if err != nil {
			fmt.Printf("⚠️  前置脚本警告: %v\n", err)
		} else {
			applyPreResult(reqCtx, result)
		}
	}

	// 通过反射调用 gRPC 方法
	start := time.Now()
	respBody, err := invokeGRPC(ctx, conn, api.Path, reqCtx.Body)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return fmt.Errorf("gRPC 调用失败: %w", err)
	}

	// 构造后置脚本上下文
	postCtx := &model.ScriptContext{
		APIID:      api.ID,
		Method:     "gRPC",
		URL:        reqCtx.URL,
		Headers:    reqCtx.Headers,
		Params:     reqCtx.Params,
		Body:       reqCtx.Body,
		StatusCode: 200,
		RespBody:   respBody,
		ElapsedMs:  elapsed,
	}

	// 执行后置脚本
	postScript := script.ResolveScript(api, s.cfg.Hooks, false)
	if postScript != "" {
		result, err := s.engine.ExecutePostScript(postScript, "post_request", postCtx)
		if err != nil {
			fmt.Printf("⚠️  后置脚本警告: %v\n", err)
		} else {
			printPostResult(result)
		}
	}

	printGRPCResponse(respBody, elapsed)
	return nil
}

// invokeGRPC 通过 gRPC 反射动态调用服务方法
func invokeGRPC(ctx context.Context, conn *grpc.ClientConn, fullMethod string, body interface{}) (interface{}, error) {
	// 解析服务名和方法名: e.g. "rbe.entity.petrel.ScheduleTask/List"
	serviceName, methodName, err := parseFullMethod(fullMethod)
	if err != nil {
		return nil, err
	}

	// 通过反射获取方法描述
	inputDesc, outputDesc, _, err := resolveMethodDescriptor(ctx, conn, serviceName, methodName)
	if err != nil {
		return nil, fmt.Errorf("解析方法描述失败: %w", err)
	}

	// 构造输入消息
	inputMsg := dynamicpb.NewMessage(inputDesc)
	if err := jsonToProto(body, inputMsg); err != nil {
		return nil, fmt.Errorf("构造请求消息失败: %w", err)
	}

	// 添加 metadata
	md := metadata.New(map[string]string{})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// 构造输出消息
	outputMsg := dynamicpb.NewMessage(outputDesc)

	// 调用
	methodPath := fmt.Sprintf("/%s/%s", serviceName, methodName)
	if err := conn.Invoke(ctx, methodPath, inputMsg, outputMsg); err != nil {
		return nil, fmt.Errorf("调用方法失败: %w", err)
	}

	// 转换响应为 JSON
	return protoToJSON(outputMsg)
}

// parseFullMethod 解析全限定方法名
func parseFullMethod(fullMethod string) (service, method string, err error) {
	// 支持 "service/Method" 或 "/service/Method" 格式
	if len(fullMethod) > 0 && fullMethod[0] == '/' {
		fullMethod = fullMethod[1:]
	}

	for i := len(fullMethod) - 1; i >= 0; i-- {
		if fullMethod[i] == '/' {
			return fullMethod[:i], fullMethod[i+1:], nil
		}
	}

	return "", "", fmt.Errorf("无效的方法名格式: %s，期望格式: service/Method", fullMethod)
}

// resolveMethodDescriptor 通过 gRPC 反射解析方法的输入输出描述
func resolveMethodDescriptor(ctx context.Context, conn *grpc.ClientConn, serviceName, methodName string) (protoreflect.MessageDescriptor, protoreflect.MessageDescriptor, protoreflect.MethodDescriptor, error) {
	refClient := reflectionpb.NewServerReflectionClient(conn)
	stream, err := refClient.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("创建反射流失败: %w", err)
	}

	// 请求文件描述
	if err := stream.Send(&reflectionpb.ServerReflectionRequest{
		MessageRequest: &reflectionpb.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: serviceName,
		},
	}); err != nil {
		return nil, nil, nil, fmt.Errorf("发送反射请求失败: %w", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("接收反射响应失败: %w", err)
	}

	fileResp, ok := resp.MessageResponse.(*reflectionpb.ServerReflectionResponse_FileDescriptorResponse)
	if !ok {
		if errResp, ok := resp.MessageResponse.(*reflectionpb.ServerReflectionResponse_ErrorResponse); ok {
			return nil, nil, nil, fmt.Errorf("反射错误 [%v]: %s", errResp.ErrorResponse.ErrorCode, errResp.ErrorResponse.ErrorMessage)
		}
		return nil, nil, nil, fmt.Errorf("反射响应类型不匹配")
	}

	// 解析文件描述符
	files := &descriptorpb.FileDescriptorSet{}
	for _, fdBytes := range fileResp.FileDescriptorResponse.FileDescriptorProto {
		fdp := &descriptorpb.FileDescriptorProto{}
		if err := proto.Unmarshal(fdBytes, fdp); err != nil {
			return nil, nil, nil, fmt.Errorf("解析文件描述符失败: %w", err)
		}
		files.File = append(files.File, fdp)
	}

	// 构建文件描述符注册表
	registry, err := protodesc.NewFiles(files)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("构建描述符注册表失败: %w", err)
	}

	// 查找服务和方法
	serviceFullName := protoreflect.FullName(serviceName)
	desc, err := registry.FindDescriptorByName(serviceFullName)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("查找服务 %s 失败: %w", serviceName, err)
	}

	serviceDesc, ok := desc.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, nil, nil, fmt.Errorf("%s 不是服务描述", serviceName)
	}

	methodDesc := serviceDesc.Methods().ByName(protoreflect.Name(methodName))
	if methodDesc == nil {
		return nil, nil, nil, fmt.Errorf("方法 %s/%s 不存在", serviceName, methodName)
	}

	return methodDesc.Input(), methodDesc.Output(), methodDesc, nil
}

// jsonToProto 将 JSON 数据转换为 proto 消息
func jsonToProto(data interface{}, msg *dynamicpb.Message) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return unmarshalJSON(jsonBytes, msg)
}

// unmarshalJSON 简单的 JSON 到 proto 的转换
func unmarshalJSON(data []byte, msg *dynamicpb.Message) error {
	// 使用 protojson 进行反序列化
	return protojsonUnmarshal(data, msg)
}

// protoToJSON 将 proto 消息转换为 JSON
func protoToJSON(msg proto.Message) (interface{}, error) {
	jsonBytes, err := protojsonMarshal(msg)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// printGRPCResponse 输出 gRPC 响应
func printGRPCResponse(body interface{}, elapsed int64) {
	fmt.Printf("\n── gRPC 响应 ──────────────────────\n")
	fmt.Printf("耗时: %dms\n", elapsed)
	fmt.Printf("────────────────────────────────────\n")

	data, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		fmt.Printf("%v\n", body)
	} else {
		fmt.Println(string(data))
	}
}
