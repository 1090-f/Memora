package mcp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Encrypt 使用 AES-256-GCM 加密任意字节数据，返回 nonce+ciphertext。
func Encrypt(data []byte, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("encryption key must be 32 bytes for AES-256")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, data, nil), nil
}

// Decrypt 解密 Encrypt 产出的 nonce+ciphertext。
func Decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("encryption key must be 32 bytes for AES-256")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, encrypted := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}

// EncryptStringMap 将 map[string]string 序列化为 JSON 后整体加密。
func EncryptStringMap(m map[string]string, key []byte) ([]byte, error) {
	if len(m) == 0 {
		return nil, nil
	}
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal map: %w", err)
	}
	return Encrypt(data, key)
}

// DecryptStringMap 解密并反序列化为 map[string]string。
func DecryptStringMap(ciphertext []byte, key []byte) (map[string]string, error) {
	if len(ciphertext) == 0 {
		return nil, nil
	}
	plaintext, err := Decrypt(ciphertext, key)
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(plaintext, &m); err != nil {
		return nil, fmt.Errorf("unmarshal map: %w", err)
	}
	return m, nil
}

// EncryptStringSlice 将 []string 序列化为 JSON 后整体加密。
func EncryptStringSlice(s []string, key []byte) ([]byte, error) {
	if len(s) == 0 {
		return nil, nil
	}
	data, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("marshal slice: %w", err)
	}
	return Encrypt(data, key)
}

// DecryptStringSlice 解密并反序列化为 []string。
func DecryptStringSlice(ciphertext []byte, key []byte) ([]string, error) {
	if len(ciphertext) == 0 {
		return nil, nil
	}
	plaintext, err := Decrypt(ciphertext, key)
	if err != nil {
		return nil, err
	}
	var s []string
	if err := json.Unmarshal(plaintext, &s); err != nil {
		return nil, fmt.Errorf("unmarshal slice: %w", err)
	}
	return s, nil
}

// MaskStringMap 返回脱敏后的 map：保留 Key，Value 统一替换为 "******"。
func MaskStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	masked := make(map[string]string, len(m))
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		masked[k] = "******"
	}
	return masked
}

// DeriveKey 从配置中的 encryption key 字符串派生 32 字节 AES-256 密钥。
// 不足 32 字节时右侧补零，超过 32 字节截取前 32 字节。
func DeriveKey(rawKey string) []byte {
	key := []byte(rawKey)
	if len(key) >= 32 {
		return key[:32]
	}
	padded := make([]byte, 32)
	copy(padded, key)
	return padded
}

// SensitiveEnvKeys 匹配环境变量名中的敏感关键字，
// 用于在脱敏输出时区分哪些值需要隐藏。
func isSensitiveEnvKey(key string) bool {
	lower := strings.ToLower(key)
	for _, kw := range []string{"secret", "token", "password", "api_key", "apikey", "access_key", "private_key", "authorization"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
