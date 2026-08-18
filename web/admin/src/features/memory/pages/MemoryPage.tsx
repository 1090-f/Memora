import { useState } from 'react';
import {
  Button,
  Card,
  CardContent,
  Chip,
  Stack,
  Typography,
  IconButton,
  Switch,
  FormControlLabel,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
  Box,
  Divider,
} from '@mui/material';
import {
  Delete as DeleteIcon,
  ChatBubbleOutline as ConversationIcon,
  AccessTime as TimeIcon,
  Star as StarIcon,
} from '@mui/icons-material';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { queryKeys } from '@/api/queryKeys';
import { EmptyState } from '@/components/shared/EmptyState';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { UnavailableState } from '@/components/shared/UnavailableState';
import { listMemories, updateMemoryStatus, deleteMemory } from '../api';
import type { Memory, MemoryType, MemoryStatus } from '../types';

// 记忆类型到中文的映射
const MEMORY_TYPE_LABELS: Record<MemoryType, string> = {
  preference: '偏好',
  project: '项目',
  decision: '决策',
  goal: '目标',
  fact: '事实',
  progress: '进度',
};

// 记忆类型到颜色的映射
const MEMORY_TYPE_COLORS: Record<MemoryType, 'primary' | 'secondary' | 'success' | 'warning' | 'info' | 'error'> = {
  preference: 'primary',
  project: 'secondary',
  decision: 'success',
  goal: 'warning',
  fact: 'info',
  progress: 'error',
};

// 记忆类型过滤选项
const MEMORY_TYPE_FILTERS: Array<{ value: MemoryType | ''; label: string }> = [
  { value: '', label: '全部' },
  { value: 'preference', label: '偏好' },
  { value: 'project', label: '项目' },
  { value: 'decision', label: '决策' },
  { value: 'goal', label: '目标' },
  { value: 'fact', label: '事实' },
  { value: 'progress', label: '进度' },
];

// 格式化时间
function formatDateTime(dateString: string | null): string {
  if (!dateString) return '从未';
  const date = new Date(dateString);
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

// 格式化重要性为百分比
function formatImportance(importance: number): string {
  return `${Math.round(importance * 100)}%`;
}

// 删除确认对话框组件
function DeleteConfirmDialog({
  open,
  onClose,
  onConfirm,
  memoryContent,
}: {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  memoryContent: string;
}) {
  return (
    <Dialog open={open} onClose={onClose}>
      <DialogTitle>确认删除</DialogTitle>
      <DialogContent>
        <DialogContentText>
          确定要删除这条记忆吗？此操作不可撤销。
        </DialogContentText>
        <Typography
          sx={{
            mt: 2,
            p: 1.5,
            bgcolor: 'grey.100',
            borderRadius: 1,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            display: '-webkit-box',
            WebkitLineClamp: 2,
            WebkitBoxOrient: 'vertical',
          }}
        >
          {memoryContent}
        </Typography>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>取消</Button>
        <Button onClick={onConfirm} color="error" variant="contained">
          确认删除
        </Button>
      </DialogActions>
    </Dialog>
  );
}

// 单个记忆卡片组件
function MemoryCard({
  memory,
  onStatusChange,
  onDelete,
}: {
  memory: Memory;
  onStatusChange: (id: string, status: 'active' | 'inactive') => void;
  onDelete: (memory: Memory) => void;
}) {
  const isActive = memory.status === 'active';

  return (
    <Card variant="outlined">
      <CardContent>
        {/* 顶部：类型、状态标签和操作按钮 */}
        <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 2 }}>
          <Chip
            size="small"
            label={MEMORY_TYPE_LABELS[memory.memory_type]}
            color={MEMORY_TYPE_COLORS[memory.memory_type]}
          />
          <Chip
            size="small"
            label={isActive ? '启用' : '停用'}
            color={isActive ? 'success' : 'default'}
            variant={isActive ? 'filled' : 'outlined'}
          />
          <Box sx={{ flexGrow: 1 }} />
          <FormControlLabel
            control={
              <Switch
                size="small"
                checked={isActive}
                onChange={(e) => {
                  onStatusChange(memory.id, e.target.checked ? 'active' : 'inactive');
                }}
              />
            }
            label="启用"
          />
          <IconButton
            size="small"
            color="error"
            onClick={() => onDelete(memory)}
            title="删除"
          >
            <DeleteIcon fontSize="small" />
          </IconButton>
        </Stack>

        {/* 标题和内容 */}
        <Typography variant="h6" fontWeight={700} sx={{ mb: 1 }}>
          {memory.summary || '无标题'}
        </Typography>
        <Typography
          color="text.secondary"
          sx={{
            overflowWrap: 'anywhere',
            mb: 2,
            display: '-webkit-box',
            WebkitLineClamp: 3,
            WebkitBoxOrient: 'vertical',
            overflow: 'hidden',
          }}
        >
          {memory.content}
        </Typography>

        {/* 底部：重要性、最近召回时间、来源 */}
        <Divider sx={{ my: 1.5 }} />
        <Stack direction="row" spacing={2} flexWrap="wrap" useFlexGap>
          <Stack direction="row" alignItems="center" spacing={0.5}>
            <StarIcon fontSize="small" color="warning" />
            <Typography variant="body2" color="text.secondary">
              重要性: {formatImportance(memory.importance)}
            </Typography>
          </Stack>

          <Stack direction="row" alignItems="center" spacing={0.5}>
            <TimeIcon fontSize="small" color="info" />
            <Typography variant="body2" color="text.secondary">
              最近召回: {formatDateTime(memory.last_accessed_at)}
            </Typography>
          </Stack>

          {memory.source_conversation_id && (
            <Stack direction="row" alignItems="center" spacing={0.5}>
              <ConversationIcon fontSize="small" color="action" />
              <Typography variant="body2" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
                来源: {memory.source_conversation_id.slice(0, 8)}...
              </Typography>
            </Stack>
          )}
        </Stack>
      </CardContent>
    </Card>
  );
}

// 主页面内容组件
export function MemoryPageContent({ status }: { status: CapabilityStatus }) {
  const enabled = status === 'available';
  const queryClient = useQueryClient();

  // 状态管理
  const [typeFilter, setTypeFilter] = useState<MemoryType | ''>('');
  const [page, setPage] = useState(1);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [memoryToDelete, setMemoryToDelete] = useState<Memory | null>(null);

  // 查询记忆列表
  const query = useQuery({
    queryKey: [...queryKeys.memories, typeFilter, page],
    queryFn: () =>
      listMemories({
        page,
        page_size: 20,
        memory_type: typeFilter || undefined,
      }),
    enabled,
  });

  // 更新状态的 mutation
  const statusMutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: 'active' | 'inactive' }) =>
      updateMemoryStatus(id, status),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.memories });
    },
  });

  // 删除的 mutation
  const deleteMutation = useMutation({
    mutationFn: deleteMemory,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.memories });
      setDeleteDialogOpen(false);
      setMemoryToDelete(null);
    },
  });

  // 处理状态切换
  const handleStatusChange = (id: string, newStatus: 'active' | 'inactive') => {
    statusMutation.mutate({ id, status: newStatus });
  };

  // 处理删除点击
  const handleDeleteClick = (memory: Memory) => {
    setMemoryToDelete(memory);
    setDeleteDialogOpen(true);
  };

  // 确认删除
  const handleConfirmDelete = () => {
    if (memoryToDelete) {
      deleteMutation.mutate(memoryToDelete.id);
    }
  };

  // 取消删除
  const handleCancelDelete = () => {
    setDeleteDialogOpen(false);
    setMemoryToDelete(null);
  };

  // 获取总页数
  const totalPages = query.data ? Math.ceil(query.data.total / 20) : 1;

  return (
    <Stack spacing={3}>
      {/* 页面标题 */}
      <Stack direction="row" alignItems="center">
        <Typography component="h2" variant="h5" fontWeight={750} sx={{ flexGrow: 1 }}>
          长期记忆
        </Typography>
        <Button disabled={!enabled}>更改记忆状态</Button>
      </Stack>

      {/* 功能状态提示 */}
      {!enabled && (
        <UnavailableState
          title="长期记忆后端待接入"
          description="后端待接入；当前不会加载或修改由 Agent 形成的记忆。"
          capability="memory"
        />
      )}

      {/* 分类导航栏 */}
      {enabled && (
        <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
          {MEMORY_TYPE_FILTERS.map((filter) => (
            <Chip
              key={filter.value}
              label={filter.label}
              onClick={() => {
                setTypeFilter(filter.value);
                setPage(1);
              }}
              color={filter.value === typeFilter ? 'primary' : 'default'}
              variant={filter.value === typeFilter ? 'filled' : 'outlined'}
              sx={{ cursor: 'pointer' }}
            />
          ))}
        </Stack>
      )}

      {/* 加载状态 */}
      {enabled && query.isPending && <LoadingState label="正在加载长期记忆" />}

      {/* 错误状态 */}
      {enabled && query.error && (
        <ErrorState error={query.error as Error} onRetry={() => void query.refetch()} />
      )}

      {/* 空状态 */}
      {enabled && query.data?.items.length === 0 && (
        <EmptyState
          title="暂无长期记忆"
          description={
            typeFilter
              ? `没有找到类型为"${MEMORY_TYPE_LABELS[typeFilter]}"的记忆`
              : 'Agent 完成问答后可能自动提取记忆。'
          }
        />
      )}

      {/* 记忆列表 */}
      {query.data?.items.map((memory) => (
        <MemoryCard
          key={memory.id}
          memory={memory}
          onStatusChange={handleStatusChange}
          onDelete={handleDeleteClick}
        />
      ))}

      {/* 分页控制 */}
      {enabled && query.data && query.data.total > 20 && (
        <Stack direction="row" justifyContent="center" spacing={2} sx={{ mt: 3 }}>
          <Button
            variant="outlined"
            disabled={page === 1}
            onClick={() => setPage((p) => Math.max(1, p - 1))}
          >
            上一页
          </Button>
          <Typography variant="body2" sx={{ alignSelf: 'center' }}>
            第 {page} / {totalPages} 页 (共 {query.data.total} 条)
          </Typography>
          <Button
            variant="outlined"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => p + 1)}
          >
            下一页
          </Button>
        </Stack>
      )}

      {/* 删除确认对话框 */}
      <DeleteConfirmDialog
        open={deleteDialogOpen}
        onClose={handleCancelDelete}
        onConfirm={handleConfirmDelete}
        memoryContent={memoryToDelete?.content || ''}
      />
    </Stack>
  );
}

// 主页面组件
export function MemoryPage() {
  return <MemoryPageContent status={capabilities.memory} />;
}
