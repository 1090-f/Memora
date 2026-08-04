import { useTiptap } from '@ctzhian/tiptap';
import { Box, Skeleton, Stack } from '@mui/material';
import { useOutletContext } from 'react-router-dom';
import { WrapContext } from '..';
import Header from './Header';
import Toolbar from './Toolbar';
import { IconAShijian2, IconZiti } from '@panda-wiki/icons';

const LoadingEditorWrap = () => {
  const { catalogOpen } = useOutletContext<WrapContext>();

  const editorRef = useTiptap({
    editable: false,
    content: '',
    exclude: ['invisibleCharacters', 'youtube', 'mention'],
    baseUrl: window.__BASENAME__ || '',
  });

  return (
    <Box>
      <Box
        sx={{
          position: 'fixed',
          top: 0,
          left: catalogOpen ? 292 : 0,
          right: 0,
          zIndex: 2,
          bgcolor: 'background.default',
          transition: 'left 0.3s ease-in-out',
        }}
      >
        <Header
          edit={false}
          detail={{}}
          updateDetail={() => {}}
          handleSave={() => {}}
          handleExport={() => {}}
        />
        <Toolbar editorRef={editorRef} />
      </Box>
      <Box>
        <Box
          sx={{
            p: '72px 72px 150px',
            mt: '102px',
            mx: 'auto',
            maxWidth: 892,
            minWidth: '386px',
          }}
        >
          <Stack direction={'row'} alignItems={'center'} gap={1} sx={{ mb: 2 }}>
            <Skeleton variant='text' width={36} height={36} />
            <Skeleton variant='text' width={300} height={36} />
          </Stack>
          <Stack direction={'row'} alignItems={'center'} gap={2} sx={{ mb: 4 }}>
            <Stack direction={'row'} alignItems={'center'} gap={0.5}>
              <IconAShijian2 sx={{ color: 'text.tertiary', fontSize: 12 }} />
              <Skeleton variant='text' width={130} height={24} />
            </Stack>
            <Stack direction={'row'} alignItems={'center'} gap={0.5}>
              <IconZiti sx={{ color: 'text.tertiary', fontSize: 12 }} />
              <Skeleton variant='text' width={80} height={24} />
            </Stack>
          </Stack>
          <Stack
            gap={1}
            sx={{
              minHeight: 'calc(100vh - 432px)',
            }}
          >
            <Skeleton variant='text' height={24} />
            <Skeleton variant='text' width={300} height={24} />
            <Skeleton variant='text' height={24} />
            <Skeleton variant='text' height={24} />
            <Skeleton variant='text' width={600} height={24} />
          </Stack>
        </Box>
      </Box>
    </Box>
  );
};

export default LoadingEditorWrap;
