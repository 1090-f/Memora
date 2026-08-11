package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/1090-f/Memora/pkg/config"
	"github.com/1090-f/Memora/pkg/logger"
)

const documentParserHealthInterval = 500 * time.Millisecond

// documentParserProcess manages the optional local document-parser child process.
// An already-running parser is reused and never stopped by Memora.
type documentParserProcess struct {
	cfg config.DocumentParserConfig

	cmd     *exec.Cmd
	done    chan struct{}
	errMu   sync.Mutex
	cmdErr  error
	started bool

	closeOnce sync.Once
	closeErr  error
}

func newDocumentParserProcess(cfg config.DocumentParserConfig) *documentParserProcess {
	return &documentParserProcess{cfg: cfg}
}

func (p *documentParserProcess) Ensure(ctx context.Context) error {
	if !p.cfg.AutoStart {
		return nil
	}
	if err := validateLocalParserURL(p.cfg.BaseURL); err != nil {
		return err
	}

	client := newDocumentParserHTTPClient()
	ready, readyErr := parserReady(ctx, client, p.healthURL("/health/ready"))
	if readyErr != nil {
		return fmt.Errorf("document-parser 初始化失败: %w", readyErr)
	}
	if ready {
		return nil
	}

	waitCtx, cancel := context.WithTimeout(ctx, p.cfg.AutoStartTimeout)
	defer cancel()

	// A live parser may still be loading Docling models. Reuse it instead of
	// attempting to bind a second process to the same port.
	if isHealthy(ctx, client, p.healthURL("/health/live")) {
		if logger.GetLogger() != nil {
			logger.Info("document-parser 已在运行，等待模型初始化完成")
		}
		return p.waitUntilReady(waitCtx, client)
	}
	if err := waitCtx.Err(); err != nil {
		return fmt.Errorf("启动 document-parser 已取消: %w", err)
	}

	workingDirectory, err := filepath.Abs(p.cfg.AutoStartWorkingDirectory)
	if err != nil {
		return fmt.Errorf("解析 document-parser 工作目录失败: %w", err)
	}
	info, err := os.Stat(workingDirectory)
	if err != nil {
		return fmt.Errorf("document-parser 工作目录不可用 %q: %w", workingDirectory, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("document-parser 工作目录不是目录: %q", workingDirectory)
	}

	commandPath, err := exec.LookPath(p.cfg.AutoStartCommand)
	if err != nil {
		return fmt.Errorf("找不到 document-parser 启动命令 %q: %w", p.cfg.AutoStartCommand, err)
	}
	p.cmd = exec.Command(commandPath, p.cfg.AutoStartArgs...)
	p.cmd.Dir = workingDirectory
	p.cmd.Env = parserProcessEnvironment(p.cfg)
	p.cmd.Stdout = os.Stdout
	p.cmd.Stderr = os.Stderr
	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("启动 document-parser 失败: %w", err)
	}
	p.started = true
	p.done = make(chan struct{})
	go func() {
		err := p.cmd.Wait()
		p.errMu.Lock()
		p.cmdErr = err
		p.errMu.Unlock()
		close(p.done)
	}()

	if logger.GetLogger() != nil {
		logger.Infof("document-parser 已启动，PID=%d，等待服务就绪", p.cmd.Process.Pid)
	}
	if err := p.waitUntilReady(waitCtx, client); err != nil {
		_ = p.Close()
		return err
	}
	if logger.GetLogger() != nil {
		logger.Info("document-parser 已就绪")
	}
	return nil
}

func (p *documentParserProcess) Health(ctx context.Context) error {
	if p == nil {
		return errors.New("document-parser 未初始化")
	}
	ready, err := parserReady(ctx, newDocumentParserHTTPClient(), p.healthURL("/health/ready"))
	if err != nil {
		return err
	}
	if ready {
		return nil
	}
	select {
	case <-p.processDone():
		return fmt.Errorf("document-parser 进程已退出: %w", p.processError())
	default:
		return errors.New("document-parser 未就绪")
	}
}

func (p *documentParserProcess) Close() error {
	p.closeOnce.Do(func() {
		if !p.started || p.cmd == nil || p.cmd.Process == nil {
			return
		}
		select {
		case <-p.done:
			return
		default:
		}

		if runtime.GOOS == "windows" {
			// uv may spawn Python as a child. Kill the exact process tree so the
			// parser does not remain bound to port 5001 after Memora exits.
			killErr := exec.Command("taskkill", "/PID", strconv.Itoa(p.cmd.Process.Pid), "/T", "/F").Run()
			if killErr != nil {
				if fallbackErr := p.cmd.Process.Kill(); fallbackErr == nil {
					killErr = nil
				} else {
					killErr = errors.Join(killErr, fallbackErr)
				}
			}
			p.closeErr = killErr
		} else {
			p.closeErr = p.cmd.Process.Signal(os.Interrupt)
		}

		select {
		case <-p.done:
		case <-time.After(5 * time.Second):
			p.closeErr = errors.Join(p.closeErr, p.cmd.Process.Kill())
			select {
			case <-p.done:
			case <-time.After(5 * time.Second):
				p.closeErr = errors.Join(p.closeErr, errors.New("等待 document-parser 退出超时"))
			}
		}
	})
	return p.closeErr
}

func (p *documentParserProcess) waitUntilReady(ctx context.Context, client *http.Client) error {
	ticker := time.NewTicker(documentParserHealthInterval)
	defer ticker.Stop()
	for {
		ready, err := parserReady(ctx, client, p.healthURL("/health/ready"))
		if err != nil {
			return fmt.Errorf("document-parser 初始化失败: %w", err)
		}
		if ready {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待 document-parser 就绪超时: %w", ctx.Err())
		case <-p.processDone():
			return fmt.Errorf("document-parser 在就绪前退出: %w", p.processError())
		case <-ticker.C:
		}
	}
}

func (p *documentParserProcess) processDone() <-chan struct{} {
	if p.done != nil {
		return p.done
	}
	return nil
}

func (p *documentParserProcess) processError() error {
	p.errMu.Lock()
	defer p.errMu.Unlock()
	if p.cmdErr == nil {
		return errors.New("进程已退出")
	}
	return p.cmdErr
}

func (p *documentParserProcess) healthURL(path string) string {
	return strings.TrimRight(p.cfg.BaseURL, "/") + path
}

func isHealthy(ctx context.Context, client *http.Client, endpoint string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
}

func newDocumentParserHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func parserReady(ctx context.Context, client *http.Client, endpoint string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return true, nil
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		return false, nil
	}
	var payload struct {
		Status string `json:"status"`
		Detail string `json:"detail"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16*1024)).Decode(&payload); err != nil {
		return false, nil
	}
	if payload.Status == "error" {
		if payload.Detail == "" {
			payload.Detail = "模型初始化失败"
		}
		return false, errors.New(payload.Detail)
	}
	return false, nil
}

func validateLocalParserURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("document-parser base_url 无效: %w", err)
	}
	host := parsed.Hostname()
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || host == "" {
		return errors.New("document-parser base_url 必须是完整 HTTP URL")
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("自动启动仅支持本机 document-parser 地址，当前地址为 %q", rawURL)
	}
	return nil
}

func parserProcessEnvironment(cfg config.DocumentParserConfig) []string {
	values := make(map[string]string, len(cfg.AutoStartEnvironment)+2)
	for key, value := range cfg.AutoStartEnvironment {
		values[key] = value
	}
	values["DOCUMENT_PARSER_MAX_FILE_BYTES"] = strconv.FormatInt(cfg.MaxFileBytes, 10)
	values["DOCUMENT_PARSER_MAX_ASSET_BYTES"] = strconv.FormatInt(cfg.MaxAssetBytes, 10)

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	environment := os.Environ()
	for _, key := range keys {
		environment = append(environment, key+"="+values[key])
	}
	return environment
}
