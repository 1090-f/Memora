import { Typography } from '@mui/material';

export function TextViewer({ content }: { content: string }) {
  return (
    <Typography component="pre" sx={{ m: 0, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', fontFamily: 'inherit', lineHeight: 1.75 }}>
      {content}
    </Typography>
  );
}
