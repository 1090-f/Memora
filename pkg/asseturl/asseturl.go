// Package asseturl 提供文档资产下载 URL 的签名与校验。
//
// 浏览器 <img> 标签无法携带 Authorization header，资产 URL 因此改为
// HMAC 签名 + 过期时间（类似预签名 URL），由路由单独校验，不依赖 Bearer 认证。
package asseturl

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DefaultTTL 是签名 URL 的有效期。
const DefaultTTL = 6 * time.Hour

// Sign 为文档资产生成签名参数（exp 与 sig）。
// 签名内容 = documentID|assetID|exp，使用 HMAC-SHA256。
func Sign(secret, documentID, assetID string, ttl time.Duration) (exp, sig string, err error) {
	if strings.TrimSpace(secret) == "" {
		return "", "", fmt.Errorf("资产签名密钥未配置")
	}
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	expires := time.Now().Add(ttl).Unix()
	return strconv.FormatInt(expires, 10), compute(secret, documentID, assetID, expires), nil
}

// Verify 校验资产签名参数是否有效（过期或伪造返回 false）。
func Verify(secret, documentID, assetID, exp, sig string) bool {
	if strings.TrimSpace(secret) == "" {
		return false
	}
	expires, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || expires <= 0 {
		return false
	}
	if time.Now().Unix() > expires {
		return false
	}
	return hmac.Equal([]byte(compute(secret, documentID, assetID, expires)), []byte(sig))
}

// BuildAssetURL 构造带签名的资产下载 URL（相对路径，含过期与签名参数）。
func BuildAssetURL(secret, documentID, assetID string, ttl time.Duration) (string, error) {
	exp, sig, err := Sign(secret, documentID, assetID, ttl)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("/api/v1/documents/%s/assets/%s?exp=%s&sig=%s",
		documentID, assetID, exp, sig), nil
}

func compute(secret, documentID, assetID string, expires int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(documentID))
	_, _ = mac.Write([]byte{'|'})
	_, _ = mac.Write([]byte(assetID))
	_, _ = mac.Write([]byte{'|'})
	_, _ = mac.Write([]byte(strconv.FormatInt(expires, 10)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
