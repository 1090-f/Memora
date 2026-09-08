package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/1090-f/Memora/internal/app"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/service/rag/chunking"
	"github.com/1090-f/Memora/internal/service/rag/evaluation"
	"github.com/1090-f/Memora/pkg/config"
	"github.com/1090-f/Memora/pkg/database"
)

type options struct {
	dataset, output, baseline, userID, kbID string
	mode, ks, documentIDs                   string
	topK                                    int
	experimentName, gitCommit, searchConfig string
	validateOnly                            bool
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	var opts options
	flags := flag.NewFlagSet("memora-eval-retrieval", flag.ContinueOnError)
	flags.SetOutput(stdout)
	flags.StringVar(&opts.dataset, "dataset", "", "金标 JSON 文件（必填）")
	flags.StringVar(&opts.output, "output", "", "结果 JSON 文件；省略时输出到 stdout")
	flags.StringVar(&opts.baseline, "baseline", "", "可选的基线 RunResult JSON")
	flags.StringVar(&opts.userID, "user-id", "", "用户 ID（必填）")
	flags.StringVar(&opts.kbID, "kb-id", "", "知识库 ID（必填）")
	flags.StringVar(&opts.mode, "mode", "hybrid", "keyword、vector 或 hybrid")
	flags.StringVar(&opts.ks, "ks", "1,3,5,10", "逗号分隔的评估 K")
	flags.IntVar(&opts.topK, "top-k", 10, "每题最终返回数量（最大 20）")
	flags.StringVar(&opts.documentIDs, "document-ids", "", "可选的逗号分隔文档 ID")
	flags.StringVar(&opts.searchConfig, "search-config", "", "可选 SearchConfig JSON；省略时使用知识库配置")
	flags.StringVar(&opts.experimentName, "name", "", "实验名称")
	flags.StringVar(&opts.gitCommit, "git-commit", "", "代码提交标识")
	flags.BoolVar(&opts.validateOnly, "validate-only", false, "只校验金标文件，不连接数据库")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if opts.dataset == "" {
		return errors.New("必须指定 --dataset")
	}
	mode := contracts.RetrievalMode(opts.mode)
	if mode != contracts.RetrievalKeyword && mode != contracts.RetrievalVector && mode != contracts.RetrievalHybrid {
		return fmt.Errorf("--mode 必须是 keyword、vector 或 hybrid")
	}
	ks, err := parsePositiveInts(opts.ks)
	if err != nil {
		return fmt.Errorf("解析 --ks: %w", err)
	}
	if opts.topK <= 0 || opts.topK > 20 {
		return fmt.Errorf("--top-k 必须在 1~20")
	}
	for _, k := range ks {
		if k > 20 {
			return fmt.Errorf("--ks 中的 K 不能超过生产检索上限 20")
		}
	}

	datasetFile, err := os.Open(opts.dataset)
	if err != nil {
		return fmt.Errorf("打开金标数据: %w", err)
	}
	defer datasetFile.Close()
	dataset, err := evaluation.LoadDataset(datasetFile)
	if err != nil {
		return err
	}
	if opts.validateOnly {
		_, err = fmt.Fprintf(stdout, "金标校验通过：%s（%d 条）\n", dataset.Name, len(dataset.Cases))
		return err
	}
	if opts.userID == "" || opts.kbID == "" {
		return errors.New("运行评估必须指定 --user-id 和 --kb-id")
	}

	runConfig := evaluation.RunConfig{
		UserID: contracts.ID(opts.userID), KnowledgeBaseID: contracts.ID(opts.kbID), Mode: mode,
		DocumentIDs: parseIDs(opts.documentIDs), TopK: opts.topK, Ks: ks,
		ExperimentName: opts.experimentName, GitCommit: opts.gitCommit,
	}
	if opts.searchConfig != "" {
		searchConfig, loadErr := loadSearchConfig(opts.searchConfig)
		if loadErr != nil {
			return loadErr
		}
		runConfig.SearchConfig = searchConfig
		runConfig.OverrideSearchConfig = true
	}

	cfg, err := config.LoadDatabase("")
	if err != nil {
		return err
	}
	db, err := database.InitPostgres(ctx, &cfg.Database)
	if err != nil {
		return err
	}
	defer database.ClosePostgres(db)
	retrieval, err := app.NewRetrievalServiceForDatabase(cfg, db)
	if err != nil {
		return err
	}
	result, err := evaluation.NewRunner(retrieval, chunking.NewHeuristicTokenizer()).Run(ctx, dataset, runConfig)
	if err != nil {
		return err
	}
	if opts.baseline != "" {
		baseline, loadErr := loadRunResult(opts.baseline)
		if loadErr != nil {
			return loadErr
		}
		comparison := evaluation.Compare(baseline.Report, result.Report)
		result.Comparison = &comparison
	}
	return writeResult(opts.output, stdout, result)
}

func parsePositiveInts(value string) ([]int, error) {
	parts := strings.Split(value, ",")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		var number int
		if _, err := fmt.Sscanf(strings.TrimSpace(part), "%d", &number); err != nil || number <= 0 {
			return nil, fmt.Errorf("%q 不是正整数", part)
		}
		out = append(out, number)
	}
	return out, nil
}

func parseIDs(value string) []contracts.ID {
	var out []contracts.ID
	for _, part := range strings.Split(value, ",") {
		if id := strings.TrimSpace(part); id != "" {
			out = append(out, contracts.ID(id))
		}
	}
	return out
}

func loadSearchConfig(path string) (contracts.SearchConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return contracts.SearchConfig{}, fmt.Errorf("打开检索配置: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var value contracts.SearchConfig
	if err := decoder.Decode(&value); err != nil {
		return value, fmt.Errorf("解析检索配置: %w", err)
	}
	return value, nil
}

func loadRunResult(path string) (evaluation.RunResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return evaluation.RunResult{}, fmt.Errorf("打开基线报告: %w", err)
	}
	defer file.Close()
	var value evaluation.RunResult
	if err := json.NewDecoder(file).Decode(&value); err != nil {
		return value, fmt.Errorf("解析基线报告: %w", err)
	}
	return value, nil
}

func writeResult(path string, stdout io.Writer, result evaluation.RunResult) error {
	writer := stdout
	var file *os.File
	if path != "" {
		var err error
		file, err = os.Create(path)
		if err != nil {
			return fmt.Errorf("创建评估报告: %w", err)
		}
		defer file.Close()
		writer = file
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("写入评估报告: %w", err)
	}
	return nil
}
