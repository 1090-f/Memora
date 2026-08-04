import { alpha, Box, CircularProgress, Stack, useTheme } from '@mui/material';
import { ListDataItem } from '..';

interface StatusBadgeProps {
  status: ListDataItem['status'];
}

const StatusBadge = ({ status }: StatusBadgeProps) => {
  const theme = useTheme();

  type StatusConfigItem = {
    text: string;
    color: string;
    loading: boolean;
    bgColor?: string;
  };

  const statusConfig: Record<string, StatusConfigItem> = {
    common: {
      text: '解析中',
      color: theme.palette.text.secondary,
      loading: true,
    },
    'upload-error': {
      text: '上传失败',
      color: theme.palette.error.main,
      loading: false,
    },
    parsing: {
      text: '解析中',
      color: theme.palette.warning.main,
      loading: true,
    },
    importing: {
      text: '导入中',
      color: theme.palette.warning.main,
      loading: true,
    },
    'parse-error': {
      text: '解析失败',
      color: 'white',
      bgColor: 'error.main',
      loading: false,
    },
    'import-error': {
      text: '导入失败',
      color: 'white',
      bgColor: 'error.main',
      loading: false,
    },
    imported: {
      text: '导入成功',
      color: 'white',
      bgColor: 'success.main',
      loading: false,
    },
  };

  const config = statusConfig[status];

  if (!config) return null;

  if (config.loading) {
    return (
      <Stack
        direction='row'
        gap={1}
        alignItems='center'
        sx={{
          fontSize: 12,
          bgcolor: alpha(config.color, 0.1),
          color: config.color,
          px: 1,
          py: 0.25,
          borderRadius: 1,
        }}
      >
        <CircularProgress size={12} sx={{ color: config.color }} />
        {config.text}
      </Stack>
    );
  }

  return (
    <Box
      sx={{
        fontSize: 12,
        bgcolor: config.bgColor,
        color: config.color,
        px: 1,
        py: 0.25,
        borderRadius: 1,
      }}
    >
      {config.text}
    </Box>
  );
};

export default StatusBadge;
