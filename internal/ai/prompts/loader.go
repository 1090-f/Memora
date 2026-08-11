package prompts

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"

	"gopkg.in/yaml.v3"
)

//go:embed router.yaml
var routerYAML []byte

// PromptTemplate 表示提示词模板
type PromptTemplate struct {
	System string `yaml:"system"`
	User   string `yaml:"user"`
}

// RouterPrompt 路由提示词模板
var RouterPrompt *PromptTemplate

func init() {
	RouterPrompt = &PromptTemplate{}
	if err := yaml.Unmarshal(routerYAML, RouterPrompt); err != nil {
		panic(fmt.Sprintf("解析 router.yaml 失败: %v", err))
	}
}

// RouterPromptData 路由提示词数据
type RouterPromptData struct {
	Query               string
	ConversationHistory []ConversationMessage
	Memories            []MemoryResult
}

// ConversationMessage 对话消息
type ConversationMessage struct {
	Role    string
	Content string
}

// MemoryResult 记忆结果
type MemoryResult struct {
	Content string
}

// Render 渲染用户提示词
func (p *PromptTemplate) Render(data RouterPromptData) (string, error) {
	tmpl, err := template.New("user").Parse(p.User)
	if err != nil {
		return "", fmt.Errorf("解析模板失败: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("渲染模板失败: %w", err)
	}

	return buf.String(), nil
}
