import { Box } from '@mui/material';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

const markdownSx = {
  maxWidth: 920,
  mx: 'auto',
  px: { xs: 0.5, sm: 2, md: 4 },
  py: { xs: 1, md: 2.5 },
  color: '#25324b',
  fontSize: 15.5,
  overflowWrap: 'anywhere',
  '& > :first-of-type': { mt: 0 },
  '& > :last-child': { mb: 0 },
  '& h1, & h2, & h3, & h4, & h5, & h6': { mt: 3.5, mb: 1.3, color: '#17213b', lineHeight: 1.35, letterSpacing: 0 },
  '& h1': { fontSize: 28, pb: 1.2, borderBottom: '1px solid #e7eaf0' },
  '& h2': { fontSize: 22 },
  '& h3': { fontSize: 18 },
  '& p, & ul, & ol, & blockquote': { my: 1.4, lineHeight: 1.9 },
  '& li': { mb: 0.55 },
  '& pre': { overflowX: 'auto', p: 2, borderRadius: 1, bgcolor: '#f4f6f9', border: '1px solid #e6e9ef' },
  '& code': { fontFamily: 'Consolas, "SFMono-Regular", monospace' },
  '& :not(pre) > code': { px: 0.5, py: 0.2, borderRadius: 0.5, bgcolor: 'action.hover' },
  '& table': { display: 'block', maxWidth: '100%', overflowX: 'auto', borderCollapse: 'collapse', my: 2 },
  '& th, & td': { border: 1, borderColor: 'divider', px: 1.5, py: 1, textAlign: 'left' },
  '& th': { bgcolor: '#f5f7fa', color: '#34405a' },
  '& blockquote': { ml: 0, px: 2, py: 0.5, borderLeft: 4, borderColor: '#7b88ee', bgcolor: '#f7f8ff', color: '#59657b' },
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
