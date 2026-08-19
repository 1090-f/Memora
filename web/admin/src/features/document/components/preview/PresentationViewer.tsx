import { Box, Stack, Typography } from '@mui/material';
import type { PresentationSlide } from '../../types';
import { MarkdownViewer } from './MarkdownViewer';

export function PresentationViewer({ slides }: { slides: PresentationSlide[] }) {
  return (
    <Stack spacing={2} sx={{ maxWidth: 1120, mx: 'auto' }}>
      {slides.map((slide) => {
        const images = slide.images ?? [];
        const hasText = Boolean(slide.content.trim());
        const hasImages = images.length > 0;
        return (
          <Box key={slide.page} component="section" sx={{ border: '1px solid #e1e5eb', borderRadius: 1, overflow: 'hidden', bgcolor: '#fff' }}>
            <Box sx={{ display: 'flex', alignItems: 'center', minHeight: 40, px: 2, borderBottom: '1px solid #e8ebf0', bgcolor: '#f7f8fa' }}>
              <Typography sx={{ color: '#65708a', fontSize: 12.5, fontWeight: 700 }}>幻灯片 {slide.page}</Typography>
            </Box>
            <Box sx={{ display: 'grid', gridTemplateColumns: hasText && hasImages ? { xs: '1fr', md: 'minmax(0, 1.1fr) minmax(300px, .9fr)' } : '1fr', minHeight: 180 }}>
              {hasText && (
                <Box sx={{ minWidth: 0, p: { xs: 2, md: 3 }, borderRight: hasImages ? { md: '1px solid #e8ebf0' } : 0 }}>
                  <MarkdownViewer content={slide.content} compact />
                </Box>
              )}
              {hasImages && (
                <Box sx={{ display: 'grid', gridTemplateColumns: images.length > 1 ? 'repeat(2, minmax(0, 1fr))' : '1fr', alignContent: 'center', gap: 1, p: { xs: 1.5, md: 2 }, bgcolor: '#fafbfc' }}>
                  {images.map((image, index) => (
                    <Box key={`${image.url}-${index}`} component="a" href={image.url} target="_blank" rel="noopener noreferrer" title={image.alt} sx={{ display: 'grid', placeItems: 'center', minWidth: 0, height: images.length > 1 ? 180 : 300, p: 1, bgcolor: '#fff', border: '1px solid #e4e7ec', borderRadius: 1 }}>
                      <Box component="img" src={image.url} alt={image.alt} sx={{ display: 'block', maxWidth: '100%', maxHeight: '100%', width: 'auto', height: 'auto', objectFit: 'contain' }} />
                    </Box>
                  ))}
                </Box>
              )}
            </Box>
          </Box>
        );
      })}
    </Stack>
  );
}
