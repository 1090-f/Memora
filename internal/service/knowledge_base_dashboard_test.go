package service

import (
	"testing"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/stretchr/testify/require"
)

func TestKnowledgeBaseDashboardResponse(t *testing.T) {
	today := time.Date(2026, time.August, 15, 0, 0, 0, 0, time.UTC)
	fileName := "关联关系.md"
	completedAt := today.Add(17*time.Hour + 43*time.Minute)
	snapshot := &repository.KnowledgeBaseDashboardSnapshot{
		DocumentTotal: 25, IndexedTotal: 24, ProcessingTotal: 0, FailedTotal: 1,
		HighestActiveIndexVersion: 3,
		ImportTrend: []repository.KnowledgeBaseImportTrendPoint{
			{Day: today.AddDate(0, 0, -2), Count: 4},
			{Day: today, Count: 2},
		},
		RecentTasks: []*entity.ImportTask{
			{ID: "task-1", FileName: &fileName, Status: "succeeded", CreatedAt: today, CompletedAt: &completedAt},
		},
	}

	result := knowledgeBaseDashboardResponse(snapshot, today)

	require.Equal(t, 92, result.HealthScore)
	require.Equal(t, int64(25), result.DocumentTotal)
	require.Equal(t, 3, result.HighestActiveIndexVersion)
	require.Len(t, result.ImportTrend, 7)
	require.Equal(t, int64(4), result.ImportTrend[4].Count)
	require.Equal(t, int64(2), result.ImportTrend[6].Count)
	require.Len(t, result.RecentActivities, 1)
	require.Equal(t, "文件导入完成", result.RecentActivities[0].Title)
	require.Equal(t, fileName, result.RecentActivities[0].Description)
	require.Equal(t, completedAt, result.RecentActivities[0].OccurredAt)
}

func TestKnowledgeBaseDashboardResponseEmptyKnowledgeBaseIsHealthy(t *testing.T) {
	result := knowledgeBaseDashboardResponse(&repository.KnowledgeBaseDashboardSnapshot{}, time.Date(2026, time.August, 15, 0, 0, 0, 0, time.UTC))

	require.Equal(t, 100, result.HealthScore)
	require.Len(t, result.ImportTrend, 7)
	require.Empty(t, result.RecentActivities)
}
