import { Box, Typography } from '@mui/material';

export function TextViewer({ content }: { content: string }) {
  return (
    <Box sx={{ maxWidth: 920, mx: 'auto', px: { xs: 0.5, sm: 2, md: 4 }, py: { xs: 1, md: 2.5 } }}>
      <Typography component="pre" sx={{ m: 0, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', fontFamily: 'inherit', color: '#25324b', fontSize: 15.5, lineHeight: 1.9, letterSpacing: 0 }}>
        {content}
      </Typography>
    </Box>
  );
}
