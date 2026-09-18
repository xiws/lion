package sender

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// protojsonMarshal 使用 protojson 序列化 proto 消息
func protojsonMarshal(msg proto.Message) ([]byte, error) {
	return protojson.Marshal(msg)
}

// protojsonUnmarshal 使用 protojson 反序列化到 proto 消息
func protojsonUnmarshal(data []byte, msg proto.Message) error {
	return protojson.Unmarshal(data, msg)
}

// dynamicCodec 是用于 dynamicpb 消息的 gRPC codec（预留扩展）
type dynamicCodec struct{}

func (c *dynamicCodec) Name() string {
	return "proto"
}

func (c *dynamicCodec) Marshal(v interface{}) ([]byte, error) {
	if msg, ok := v.(proto.Message); ok {
		return protojsonMarshal(msg)
	}
	return nil, fmt.Errorf("dynamicCodec: unsupported type %T", v)
}

func (c *dynamicCodec) Unmarshal(data []byte, v interface{}) error {
	if msg, ok := v.(proto.Message); ok {
		return protojsonUnmarshal(data, msg)
	}
	return fmt.Errorf("dynamicCodec: unsupported type %T", v)
}
