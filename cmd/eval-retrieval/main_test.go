package main

import (
	"reflect"
	"testing"

	"github.com/1090-f/Memora/internal/contracts"
)

func TestParsePositiveInts(t *testing.T) {
	got, err := parsePositiveInts("1, 3,10")
	if err != nil || !reflect.DeepEqual(got, []int{1, 3, 10}) {
		t.Fatalf("解析 K 失败: got=%v err=%v", got, err)
	}
	if _, err := parsePositiveInts("1,0"); err == nil {
		t.Fatal("非正整数应被拒绝")
	}
}

func TestParseIDs(t *testing.T) {
	got := parseIDs("d1, ,d2")
	if !reflect.DeepEqual(got, []contracts.ID{"d1", "d2"}) {
		t.Fatalf("解析文档 ID 失败: %v", got)
	}
}
