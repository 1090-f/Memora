package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// StdioProcessConfig 是 stdio 子进程的资源配置。
type StdioProcessConfig struct {
	StartTimeout   time.Duration // 启动超时，默认 5s
	MaxOutputBytes int64         // 单次读取输出上限，默认 1MB
	KillTimeout    time.Duration // kill 后强杀等待，默认 2s
}

func defaultStdioConfig() StdioProcessConfig {
	return StdioProcessConfig{
		StartTimeout:   5 * time.Second,
		MaxOutputBytes: 1024 * 1024,
		KillTimeout:    2 * time.Second,
	}
}

// StdioProcess 管理一个 MCP stdio 子进程的生命周期，
// 通过 stdin/stdout 进行 JSON-RPC 2.0 通信。
type StdioProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	cfg    StdioProcessConfig
	nextID int
}

// StartStdioProcess 以非交互、无 shell 方式启动一个 stdio MCP 子进程。
func StartStdioProcess(target MCPServerTarget, cfg StdioProcessConfig) (*StdioProcess, error) {
	if cfg.StartTimeout <= 0 {
		cfg.StartTimeout = 5 * time.Second
	}
	if cfg.MaxOutputBytes <= 0 {
		cfg.MaxOutputBytes = 1024 * 1024
	}
	if cfg.KillTimeout <= 0 {
		cfg.KillTimeout = 2 * time.Second
	}
	if target.Command == "" {
		return nil, errors.New("command is required for stdio transport")
	}
	// 直接传入 argv，避免 shell 解释参数。
	cmd := exec.Command(target.Command, target.Args...)
	cmd.Env = buildEnv(target.Env)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}
	cmd.Stderr = nil // 丢弃 stderr，避免干扰
	startErr := make(chan error, 1)
	go func() { startErr <- cmd.Start() }()
	select {
	case err := <-startErr:
		if err != nil {
			return nil, fmt.Errorf("start process: %w", err)
		}
	case <-time.After(cfg.StartTimeout):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return nil, errors.New("start process timeout")
	}
	return &StdioProcess{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReaderSize(stdout, int(cfg.MaxOutputBytes)),
		cfg:    cfg,
		nextID: 1,
	}, nil
}

func buildEnv(extra map[string]string) []string {
	env := append([]string{}, os.Environ()...)
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}

// Request 向子进程发送一个 JSON-RPC 请求并读取响应。
func (p *StdioProcess) Request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := p.nextID
	p.nextID++
	req := jsonrpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')
	if _, err := p.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}
	line, err := readLine(ctx, p.stdout, p.cfg.MaxOutputBytes)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	var resp jsonrpcResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	return resp.Result, nil
}

// Close 先发送 exit 通知，超时后强杀进程。
func (p *StdioProcess) Close() error {
	// 发送 shutdown 通知
	notification := jsonrpcNotification{JSONRPC: "2.0", Method: "notifications/exit"}
	data, _ := json.Marshal(notification)
	_, _ = p.stdin.Write(append(data, '\n'))
	_ = p.stdin.Close()
	done := make(chan error, 1)
	go func() { done <- p.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(p.cfg.KillTimeout):
		_ = p.cmd.Process.Kill()
		<-done
	}
	return nil
}

// readLine 从 reader 读取一行 JSON，受 context 超时控制。
func readLine(ctx context.Context, reader *bufio.Reader, maxBytes int64) ([]byte, error) {
	type result struct {
		line []byte
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := reader.ReadBytes('\n')
		if err != nil && line == nil {
			ch <- result{nil, err}
			return
		}
		// 去除末尾换行
		line = trimRightNewline(line)
		ch <- result{line, nil}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.line, r.err
	}
}

func trimRightNewline(s []byte) []byte {
	return []byte(strings.TrimRight(string(s), "\r\n"))
}

// JSON-RPC 2.0 类型

type jsonrpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonrpcNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
