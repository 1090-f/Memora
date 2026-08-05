import ScheduleOutlined from '@mui/icons-material/ScheduleOutlined';
import { Alert, AlertTitle, Chip, Stack, Typography } from '@mui/material';
import type { CapabilityKey } from '@/app/capabilities';

export function UnavailableState({
  title,
  description,
  capability,
}: {
  title: string;
  description: string;
  capability: CapabilityKey;
}) {
  return (
    <Alert severity="info" icon={<ScheduleOutlined />} sx={{ bgcolor: '#f3f3ff' }}>
      <AlertTitle>{title}</AlertTitle>
      <Typography color="text.secondary">{description}</Typography>
      <Stack direction="row" mt={2}>
        <Chip size="small" label={`后端待接入 · ${capability}`} />
      </Stack>
    </Alert>
  );
}
