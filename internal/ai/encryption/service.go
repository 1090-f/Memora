package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// Service 定义加密解密服务接口。
type Service interface {
	// Encrypt 加密明文并返回密文。
	Encrypt(plaintext string) ([]byte, error)
	// Decrypt 解密密文并返回明文。
	Decrypt(ciphertext []byte) (string, error)
	// Mask 对敏感信息进行脱敏处理。
	Mask(sensitive string) string
}

// aesService 是 Service 接口的 AES 实现。
type aesService struct {
	gcm cipher.AEAD
}

// NewService 创建一个新的加密服务实例。
func NewService(key []byte) (Service, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &aesService{gcm: gcm}, nil
}

// Encrypt 加密明文并返回密文。
func (s *aesService) Encrypt(plaintext string) ([]byte, error) {
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return s.gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// Decrypt 解密密文并返回明文。
func (s *aesService) Decrypt(ciphertext []byte) (string, error) {
	nonceSize := s.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := s.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// Mask 对敏感信息进行脱敏处理，保留前4位和后4位。
func (s *aesService) Mask(sensitive string) string {
	if len(sensitive) <= 8 {
		return "****"
	}
	return sensitive[:4] + "****" + sensitive[len(sensitive)-4:]
}

// Base64Encode 将字节数组编码为 Base64 字符串。
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode 将 Base64 字符串解码为字节数组。
func Base64Decode(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}
