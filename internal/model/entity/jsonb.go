package entity

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringSlice 是可扫描 JSON 数组的字符串切片（jsonb 列用）。
type StringSlice []string

// Scan 实现 sql.Scanner。
func (s *StringSlice) Scan(value any) error {
	if value == nil {
		*s = nil
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("StringSlice 不支持扫描类型 %T", value)
	}
	var out []string
	if err := json.Unmarshal(data, &out); err != nil {
		return fmt.Errorf("解析 JSON 数组失败: %w", err)
	}
	*s = out
	return nil
}

// Value 实现 driver.Valuer。
func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

// StringMap 是可扫描 JSON 对象的字符串映射（jsonb 列用）。
type StringMap map[string]string

// Scan 实现 sql.Scanner。
func (m *StringMap) Scan(value any) error {
	if value == nil {
		*m = nil
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("StringMap 不支持扫描类型 %T", value)
	}
	var out map[string]string
	if err := json.Unmarshal(data, &out); err != nil {
		return fmt.Errorf("解析 JSON 对象失败: %w", err)
	}
	*m = out
	return nil
}

// Value 实现 driver.Valuer。
func (m StringMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}
