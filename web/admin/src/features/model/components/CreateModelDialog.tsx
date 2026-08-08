import { useState } from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Button,
  MenuItem,
  Stack,
  FormControlLabel,
  Checkbox,
} from '@mui/material';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '@/api/queryKeys';
import { createModelConfig } from '../api';
import type { ModelType } from '../types';

const MODEL_TYPES: { value: ModelType; label: string }[] = [
  { value: 'chat', label: '聊天模型 (Chat)' },
  { value: 'embedding', label: '嵌入模型 (Embedding)' },
  { value: 'reranker', label: '重排序模型 (Reranker)' },
];

const PROVIDERS = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'deepseek', label: 'DeepSeek' },
  { value: 'zhipu', label: '智谱AI' },
  { value: 'tongyi', label: '通义千问' },
  { value: 'moonshot', label: 'Moonshot' },
  { value: 'minimax', label: 'MiniMax' },
  { value: 'baidu', label: '百度文心' },
  { value: 'cohere', label: 'Cohere' },
  { value: 'custom', label: '自定义' },
];

interface CreateModelDialogProps {
  open: boolean;
  onClose: () => void;
}

export function CreateModelDialog({ open, onClose }: CreateModelDialogProps) {
  const queryClient = useQueryClient();
  const [formData, setFormData] = useState({
    name: '',
    provider: 'openai',
    model_type: 'chat' as ModelType,
    base_url: '',
    api_key: '',
    is_default: false,
    max_tokens: '',
    temperature: '',
    vector_dimension: '',
  });

  const mutation = useMutation({
    mutationFn: createModelConfig,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.models });
      onClose();
      setFormData({
        name: '',
        provider: 'openai',
        model_type: 'chat',
        base_url: '',
        api_key: '',
        is_default: false,
        max_tokens: '',
        temperature: '',
        vector_dimension: '',
      });
    },
  });

  const handleSubmit = () => {
    mutation.mutate({
      name: formData.name,
      provider: formData.provider,
      model_type: formData.model_type,
      base_url: formData.base_url,
      api_key: formData.api_key,
      is_default: formData.is_default,
      max_tokens: formData.max_tokens ? parseInt(formData.max_tokens) : null,
      temperature: formData.temperature ? parseFloat(formData.temperature) : null,
      vector_dimension: formData.vector_dimension ? parseInt(formData.vector_dimension) : null,
      timeout_seconds: 60,
      retry_times: 2,
      supports_tool_calling: false,
      supports_streaming: true,
      enabled: true,
    });
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>新增模型配置</DialogTitle>
      <DialogContent>
        <Stack spacing={3} sx={{ mt: 1 }}>
          <TextField
            label="配置名称"
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            fullWidth
            required
            placeholder="例如：GPT-4 聊天模型"
          />

          <TextField
            select
            label="模型类型"
            value={formData.model_type}
            onChange={(e) => setFormData({ ...formData, model_type: e.target.value as ModelType })}
            fullWidth
            required
          >
            {MODEL_TYPES.map((option) => (
              <MenuItem key={option.value} value={option.value}>
                {option.label}
              </MenuItem>
            ))}
          </TextField>

          <TextField
            select
            label="提供商"
            value={formData.provider}
            onChange={(e) => setFormData({ ...formData, provider: e.target.value })}
            fullWidth
            required
          >
            {PROVIDERS.map((option) => (
              <MenuItem key={option.value} value={option.value}>
                {option.label}
              </MenuItem>
            ))}
          </TextField>

          <TextField
            label="API Base URL"
            value={formData.base_url}
            onChange={(e) => setFormData({ ...formData, base_url: e.target.value })}
            fullWidth
            required
            placeholder="https://api.openai.com/v1"
          />

          <TextField
            label="API Key"
            type="password"
            value={formData.api_key}
            onChange={(e) => setFormData({ ...formData, api_key: e.target.value })}
            fullWidth
            required
            placeholder="sk-..."
          />

          {formData.model_type === 'chat' && (
            <>
              <TextField
                label="Max Tokens"
                type="number"
                value={formData.max_tokens}
                onChange={(e) => setFormData({ ...formData, max_tokens: e.target.value })}
                fullWidth
                placeholder="4096"
              />
              <TextField
                label="Temperature"
                type="number"
                value={formData.temperature}
                onChange={(e) => setFormData({ ...formData, temperature: e.target.value })}
                fullWidth
                placeholder="0.7"
                inputProps={{ min: 0, max: 2, step: 0.1 }}
              />
            </>
          )}

          {formData.model_type === 'embedding' && (
            <TextField
              label="向量维度"
              type="number"
              value={formData.vector_dimension}
              onChange={(e) => setFormData({ ...formData, vector_dimension: e.target.value })}
              fullWidth
              placeholder="1536"
            />
          )}

          <FormControlLabel
            control={
              <Checkbox
                checked={formData.is_default}
                onChange={(e) => setFormData({ ...formData, is_default: e.target.checked })}
              />
            }
            label="设为默认配置"
          />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>取消</Button>
        <Button
          onClick={handleSubmit}
          variant="contained"
          disabled={!formData.name || !formData.base_url || !formData.api_key || mutation.isPending}
        >
          {mutation.isPending ? '创建中...' : '创建'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
