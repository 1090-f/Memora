package tools

import "github.com/1090-f/Memora/internal/contracts"

// NewBuiltinTools 显式构造内置只读工具；App 在拥有真实 Service 实现后调用此函数完成组装。
func NewBuiltinTools(retrieval contracts.RetrievalService, documents contracts.DocumentService) []Tool {
	return []Tool{NewKnowledgeSearchTool(retrieval), NewDocumentReadTool(documents)}
}

// NewBuiltinRegistry 创建并注册内置工具，避免依赖 init 或全局可变状态。
func NewBuiltinRegistry(retrieval contracts.RetrievalService, documents contracts.DocumentService) (*Registry, error) {
	registry := NewRegistry()
	if err := RegisterBuiltinTools(registry, retrieval, documents); err != nil {
		return nil, err
	}
	return registry, nil
}

// RegisterBuiltinTools 将内置工具注册到显式注册表；App 应在注入真实 Service 后调用。
func RegisterBuiltinTools(registry *Registry, retrieval contracts.RetrievalService, documents contracts.DocumentService) error {
	for _, value := range NewBuiltinTools(retrieval, documents) {
		if err := registry.Register(value); err != nil {
			return err
		}
	}
	return nil
}
