import ErrorOutlineOutlined from '@mui/icons-material/ErrorOutlineOutlined';
import { Alert, Button, Stack, Typography } from '@mui/material';
import { RequestIdCopy } from './RequestIdCopy';

export interface DisplayError {
  message: string;
  requestId?: string;
}

export function ErrorState({ error, onRetry }: { error: DisplayError; onRetry?: () => void }) {
  return (
    <Alert severity="error" icon={<ErrorOutlineOutlined />}>
      <Stack alignItems="flex-start">
        <Typography fontWeight={700}>加载失败</Typography>
        <Typography mt={0.5}>{error.message}</Typography>
        {error.requestId && <RequestIdCopy requestId={error.requestId} />}
        {onRetry && <Button onClick={onRetry} sx={{ mt: 1 }}>重试</Button>}
      </Stack>
    </Alert>
  );
}
