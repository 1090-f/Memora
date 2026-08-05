import { Chip, Paper, Stack, Typography } from '@mui/material';
import type { Document } from '../types';

export function DocumentViewer({ document }: { document: Document }) {
  return (
    <Paper variant="outlined" sx={{ p: 3, minHeight: 480, overflow: 'hidden' }}>
      <Stack direction="row" alignItems="center" spacing={1} mb={3}>
        <Typography component="h2" variant="h5" fontWeight={750}>{document.title}</Typography>
        <Chip size="small" label={document.processing_status} />
      </Stack>
      <Typography component="pre" sx={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', fontFamily: 'inherit' }}>
        {document.content}
      </Typography>
    </Paper>
  );
}
