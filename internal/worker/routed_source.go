package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// RoutedSource 聚合多个 Source，按注册顺序轮询获取任务并路由操作到对应的 Source。
type RoutedSource struct {
	mutex   sync.RWMutex
	order   []string
	sources map[string]Source
}

// NewRoutedSource 创建一个新的路由任务源实例。
func NewRoutedSource() *RoutedSource { return &RoutedSource{sources: make(map[string]Source)} }

// Register 注册一个任务类型及其对应的任务源。
func (s *RoutedSource) Register(jobType string, source Source) error {
	if jobType == "" || source == nil {
		return fmt.Errorf("job type and source are required")
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if _, exists := s.sources[jobType]; exists {
		return fmt.Errorf("worker source %q is already registered", jobType)
	}
	s.sources[jobType] = source
	s.order = append(s.order, jobType)
	return nil
}

// Reserve 按注册顺序轮询所有 Source 获取可执行任务。
func (s *RoutedSource) Reserve(ctx context.Context) (*Job, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	for _, jobType := range s.order {
		job, err := s.sources[jobType].Reserve(ctx)
		if errors.Is(err, ErrNoWork) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if job == nil {
			continue
		}
		if job.Type == "" {
			job.Type = jobType
		}
		if job.Type != jobType {
			return nil, fmt.Errorf("worker source %q returned job type %q", jobType, job.Type)
		}
		return job, nil
	}
	return nil, ErrNoWork
}

// Complete 将任务标记为已完成，路由到对应的 Source。
func (s *RoutedSource) Complete(ctx context.Context, job Job) error {
	source, err := s.source(job.Type)
	if err != nil {
		return err
	}
	return source.Complete(ctx, job)
}

// Retry 将任务标记为待重试，路由到对应的 Source。
func (s *RoutedSource) Retry(ctx context.Context, job Job, availableAt time.Time, cause error) error {
	source, err := s.source(job.Type)
	if err != nil {
		return err
	}
	return source.Retry(ctx, job, availableAt, cause)
}

// Fail 将任务标记为失败，路由到对应的 Source。
func (s *RoutedSource) Fail(ctx context.Context, job Job, cause error) error {
	source, err := s.source(job.Type)
	if err != nil {
		return err
	}
	return source.Fail(ctx, job, cause)
}

func (s *RoutedSource) source(jobType string) (Source, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	source, exists := s.sources[jobType]
	if !exists {
		return nil, fmt.Errorf("worker source %q is not registered", jobType)
	}
	return source, nil
}
