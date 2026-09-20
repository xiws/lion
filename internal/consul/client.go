package consul

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/hashicorp/consul/api"
)

// Client Consul 服务发现客户端
type Client struct {
	client *api.Client
}

// NewClient 创建 Consul 客户端
func NewClient(addr, token string) (*Client, error) {
	config := api.DefaultConfig()
	config.Address = strings.TrimRight(addr, "/")
	config.Token = token

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("创建 Consul 客户端失败: %w", err)
	}

	return &Client{client: client}, nil
}

// ServiceInstance 服务实例
type ServiceInstance struct {
	Address string
	Port    int
}

// DiscoverService 发现服务实例
func (c *Client) DiscoverService(serviceName string) ([]ServiceInstance, error) {
	entries, _, err := c.client.Health().Service(serviceName, "", true, nil)
	if err != nil {
		return nil, fmt.Errorf("查询服务 %s 失败: %w", serviceName, err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("服务 %s 没有可用实例", serviceName)
	}

	var instances []ServiceInstance
	for _, entry := range entries {
		if strings.HasPrefix(entry.Service.ID, "debug-") {
			continue
		}
		instances = append(instances, ServiceInstance{
			Address: entry.Service.Address,
			Port:    entry.Service.Port,
		})
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("服务 %s 没有可用实例", serviceName)
	}

	return instances, nil
}

// PickInstance 随机选取一个服务实例
func (c *Client) PickInstance(serviceName string) (*ServiceInstance, error) {
	instances, err := c.DiscoverService(serviceName)
	if err != nil {
		return nil, err
	}

	idx := rand.Intn(len(instances))
	return &instances[idx], nil
}

// Addr 返回服务实例的地址字符串
func (s *ServiceInstance) Addr() string {
	return fmt.Sprintf("%s:%d", s.Address, s.Port)
}
