import { alpha, Box, useTheme } from '@mui/material';
import { ListDataItem } from '..';

interface StatusBackgroundProps {
  status: ListDataItem['status'];
}

/**
 * 状态背景色组件
 */
const StatusBackground = ({ status }: StatusBackgroundProps) => {
  const theme = useTheme();

  if (status === 'imported') {
    return (
      <Box
        sx={{
          width: '100%',
          height: '100%',
          bgcolor: alpha(theme.palette.success.main, 0.1),
          position: 'absolute',
          top: 0,
          left: 0,
        }}
      />
    );
  }

  if (status.includes('error')) {
    return (
      <Box
        sx={{
          width: '100%',
          height: '100%',
          bgcolor: alpha(theme.palette.error.main, 0.05),
          position: 'absolute',
          top: 0,
          left: 0,
        }}
      />
    );
  }

  return null;
};

export default StatusBackground;
