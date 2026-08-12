import { Box } from '@mui/material';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

const markdownSx = {
  overflowWrap: 'anywhere',
  '& > :first-of-type': { mt: 0 },
  '& > :last-child': { mb: 0 },
  '& h1, & h2, & h3, & h4, & h5, & h6': { mt: 2.5, mb: 1, lineHeight: 1.35 },
  '& p, & ul, & ol, & blockquote': { my: 1.25, lineHeight: 1.75 },
  '& pre': { overflowX: 'auto', p: 1.5, borderRadius: 1, bgcolor: 'action.hover' },
  '& code': { fontFamily: 'Consolas, "SFMono-Regular", monospace' },
  '& :not(pre) > code': { px: 0.5, py: 0.2, borderRadius: 0.5, bgcolor: 'action.hover' },
  '& table': { display: 'block', maxWidth: '100%', overflowX: 'auto', borderCollapse: 'collapse', my: 2 },
  '& th, & td': { border: 1, borderColor: 'divider', px: 1.25, py: 0.75, textAlign: 'left' },
  '& blockquote': { ml: 0, pl: 2, borderLeft: 4, borderColor: 'divider', color: 'text.secondary' },
  '& img': { maxWidth: '100%', maxHeight: 480, width: 'auto', height: 'auto', cursor: 'zoom-in' },
} as const;

export function MarkdownViewer({ content }: { content: string }) {
  return (
    <Box sx={markdownSx}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          img: ({ node, ...props }) => {
            void node;
            return <img {...props} onClick={() => props.src && !props.src.startsWith('data:') && window.open(props.src, '_blank', 'noopener')} />;
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </Box>
  );
}
