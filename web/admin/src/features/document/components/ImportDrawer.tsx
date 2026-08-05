import { Button, Drawer, Stack, Typography } from '@mui/material';

export function ImportDrawer({ open, onClose, disabled }: { open: boolean; onClose: () => void; disabled: boolean }) {
  return (
    <Drawer anchor="right" open={open} onClose={onClose}>
      <Stack spacing={2} sx={{ width: 420, p: 3 }}>
        <Typography variant="h6">导入文档</Typography>
        <Typography color="text.secondary">支持 md、txt、pdf、docx 文件与安全的 HTTP/HTTPS URL。</Typography>
        <Button variant="contained" disabled={disabled}>选择文件</Button>
      </Stack>
    </Drawer>
  );
}
