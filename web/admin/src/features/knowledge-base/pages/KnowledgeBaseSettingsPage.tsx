import { Button, Paper, Stack, TextField, Typography } from '@mui/material';
import { UnavailableState } from '@/components/shared/UnavailableState';

export function KnowledgeBaseSettingsPage() {
  return (
    <Stack spacing={3} maxWidth={760}>
      <Typography component="h2" variant="h5" fontWeight={750}>知识库设置</Typography>
      <UnavailableState
        title="配置接口待接入"
        description="搜索和 Agent 配置的字段已按 Memora 契约预留，后端可用前不会提交更改。"
        capability="knowledgeBase"
      />
      <Paper variant="outlined" sx={{ p: 3 }}>
        <Stack spacing={2}>
          <TextField label="知识库名称" disabled />
          <TextField label="描述" multiline minRows={3} disabled />
          <Button variant="contained" disabled>保存设置</Button>
        </Stack>
      </Paper>
    </Stack>
  );
}
