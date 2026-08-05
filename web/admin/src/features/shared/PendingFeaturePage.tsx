import { Paper, Stack, Typography } from '@mui/material';
import type { CapabilityKey } from '@/app/capabilities';
import { UnavailableState } from '@/components/shared/UnavailableState';

export function PendingFeaturePage({
  title,
  description,
  capability,
}: {
  title: string;
  description: string;
  capability: CapabilityKey;
}) {
  return (
    <Stack spacing={3}>
      <Paper variant="outlined" sx={{ p: 3 }}>
        <Typography component="h2" variant="h5" fontWeight={750}>{title}</Typography>
        <Typography color="text.secondary" mt={1}>{description}</Typography>
      </Paper>
      <UnavailableState
        title={`${title}后端能力尚未开放`}
        description="前端页面结构已就绪；对应 Go 路由实现并通过契约验证后再启用真实请求。"
        capability={capability}
      />
    </Stack>
  );
}
