package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestValidateLangfuse(t *testing.T) {
	valid := LangfuseConfig{
		Enabled:   true,
		Host:      "http://localhost:3001",
		PublicKey: "pk-lf-xxx",
		SecretKey: "sk-lf-xxx",
	}

	t.Run("禁用时不校验", func(t *testing.T) {
		if err := (Config{Langfuse: LangfuseConfig{}}).validateLangfuse(); err != nil {
			t.Fatalf("disabled 配置不应报错: %v", err)
		}
	})

	t.Run("合法配置通过", func(t *testing.T) {
		if err := (Config{Langfuse: valid}).validateLangfuse(); err != nil {
			t.Fatalf("合法配置不应报错: %v", err)
		}
	})

	t.Run("缺 host", func(t *testing.T) {
		c := valid
		c.Host = ""
		if err := (Config{Langfuse: c}).validateLangfuse(); err == nil {
			t.Fatal("缺 host 应报错")
		}
	})

	t.Run("缺 public_key", func(t *testing.T) {
		c := valid
		c.PublicKey = ""
		if err := (Config{Langfuse: c}).validateLangfuse(); err == nil {
			t.Fatal("缺 public_key 应报错")
		}
	})

	t.Run("缺 secret_key", func(t *testing.T) {
		c := valid
		c.SecretKey = ""
		if err := (Config{Langfuse: c}).validateLangfuse(); err == nil {
			t.Fatal("缺 secret_key 应报错")
		}
	})

	t.Run("host 无 scheme", func(t *testing.T) {
		c := valid
		c.Host = "localhost:3001"
		if err := (Config{Langfuse: c}).validateLangfuse(); err == nil {
			t.Fatal("host 无 scheme 应报错")
		}
	})

	t.Run("host scheme 非法", func(t *testing.T) {
		c := valid
		c.Host = "ftp://localhost:3001"
		if err := (Config{Langfuse: c}).validateLangfuse(); err == nil {
			t.Fatal("host scheme 非 http(s) 应报错")
		}
	})
}

// TestLangfuseEnvBindings 只调 bindEnvironment、不调 setDefaults —— 这样 key 不在 AllKeys 里，
// AutomaticEnv 无法兜底，才能测出「漏 BindEnv」。这是方案 7.3 指定的有效写法。
func TestLangfuseEnvBindings(t *testing.T) {
	t.Setenv("MEMORA_LANGFUSE_ENABLED", "true")
	t.Setenv("MEMORA_LANGFUSE_HOST", "http://probe:3001")

	v := viper.New()
	v.SetEnvPrefix("MEMORA")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	bindEnvironment(v) // 刻意不调 setDefaults

	if got := v.GetString("langfuse.enabled"); got != "true" {
		t.Fatalf("langfuse.enabled 未绑定环境变量，实际 %q", got)
	}
	if got := v.GetString("langfuse.host"); got != "http://probe:3001" {
		t.Fatalf("langfuse.host 未绑定环境变量，实际 %q", got)
	}
}
