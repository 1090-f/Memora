import ContentCopyOutlined from '@mui/icons-material/ContentCopyOutlined';
import { Button, Stack, Typography } from '@mui/material';

export function RequestIdCopy({ requestId }: { requestId: string }) {
  const copy = () => void navigator.clipboard?.writeText(requestId);

  return (
    <Stack direction="row" alignItems="center" spacing={1} mt={2}>
      <Typography variant="caption" color="text.secondary" sx={{ overflowWrap: 'anywhere' }}>
        请求 ID：{requestId}
      </Typography>
      <Button size="small" startIcon={<ContentCopyOutlined />} onClick={copy}>
        复制
      </Button>
    </Stack>
  );
}
