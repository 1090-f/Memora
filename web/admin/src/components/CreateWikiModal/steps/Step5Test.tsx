import { Box, Stack } from '@mui/material';
import test from '@/assets/images/init/test.png';

const Step5Test = () => {
  return (
    <Stack gap={2} sx={{ textAlign: 'center', py: 4 }}>
      <Box component='img' src={test} sx={{ width: '100%' }}></Box>
    </Stack>
  );
};

export default Step5Test;
