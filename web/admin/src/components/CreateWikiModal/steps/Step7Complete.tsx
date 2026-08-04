import { useMemo } from 'react';
import { Box, Stack, Button } from '@mui/material';
import complete from '@/assets/images/init/complete.png';
import { useAppSelector } from '@/store';

const Step7Complete = () => {
  const { kbDetail } = useAppSelector(state => state.config);

  const wikiUrl = useMemo(() => {
    if (!kbDetail) return '';
    if (kbDetail.access_settings?.base_url) {
      return kbDetail.access_settings.base_url;
    } else {
      let defaultUrl: string = '';
      const host = kbDetail.access_settings?.hosts?.[0] || '';
      if (!host) return '';
      if (
        kbDetail.access_settings?.ssl_ports &&
        kbDetail.access_settings?.ssl_ports.length > 0
      ) {
        defaultUrl = kbDetail.access_settings.ssl_ports.includes(443)
          ? `https://${host}`
          : `https://${host}:${kbDetail.access_settings.ssl_ports[0]}`;
      } else if (
        kbDetail.access_settings?.ports &&
        kbDetail.access_settings?.ports.length > 0
      ) {
        defaultUrl = kbDetail.access_settings.ports.includes(80)
          ? `http://${host}`
          : `http://${host}:${kbDetail.access_settings.ports[0]}`;
      }
      return defaultUrl;
    }
  }, [kbDetail]);

  return (
    <Stack
      gap={2}
      alignItems='center'
      justifyContent='center'
      sx={{ height: '100%' }}
    >
      <Box component='img' src={complete} sx={{ width: 274 }}></Box>
      <Box sx={{ fontSize: 14, color: 'text.tertiary' }}>配置完成</Box>
      <Button
        variant='contained'
        onClick={() => {
          if (wikiUrl) {
            window.open(wikiUrl, '_blank');
          }
        }}
      >
        访问 WIKI 网站
      </Button>
    </Stack>
  );
};

export default Step7Complete;
