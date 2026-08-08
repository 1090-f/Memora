package contracts

// TokenCounter 定义计算文本 Token 数量的接口。
type TokenCounter interface {
	// Count 计算文本的 Token 数。
	Count(text string) int
}
