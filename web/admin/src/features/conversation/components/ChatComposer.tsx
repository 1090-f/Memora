import AttachFileOutlined from '@mui/icons-material/AttachFileOutlined';
import AutoAwesomeOutlined from '@mui/icons-material/AutoAwesomeOutlined';
import CodeOutlined from '@mui/icons-material/CodeOutlined';
import ImageOutlined from '@mui/icons-material/ImageOutlined';
import SendOutlined from '@mui/icons-material/SendOutlined';
import StopCircleOutlined from '@mui/icons-material/StopCircleOutlined';
import { Box, Button, IconButton, Stack, TextField } from '@mui/material';
import type { FormEvent } from 'react';

export function ChatComposer({ draft, disabled, streaming, onDraftChange, onSend, onStop }: {
  draft: string;
  disabled: boolean;
  streaming: boolean;
  onDraftChange: (value: string) => void;
  onSend: () => void;
  onStop: () => void;
}) {
  const submit = (event: FormEvent) => { event.preventDefault(); if (!disabled && draft.trim()) onSend(); };
  return (
    <Box component="form" sx={{ p: 1.5, borderTop: '1px solid #e5e8ef', bgcolor: '#fff' }} onSubmit={submit}>
      <Box sx={{ border: '1px solid #dce1ea', borderRadius: 2.5, p: 1.2, transition: 'border-color .2s, box-shadow .2s', '&:focus-within': { borderColor: '#7180ef', boxShadow: '0 0 0 4px rgba(82,95,230,.08)' } }}>
        <TextField
          placeholder="输入问题，支持基于知识库提问..."
          value={draft}
          onChange={(event) => onDraftChange(event.target.value)}
          disabled={disabled}
          fullWidth
          multiline
          minRows={2}
          maxRows={5}
          variant="standard"
          slotProps={{ input: { disableUnderline: true } }}
        />
        <Stack direction="row" alignItems="center" sx={{ mt: 0.6 }}>
          <IconButton size="small" aria-label="添加附件"><AttachFileOutlined sx={{ fontSize: 20 }} /></IconButton>
          <IconButton size="small" aria-label="插入代码"><CodeOutlined sx={{ fontSize: 20 }} /></IconButton>
          <IconButton size="small" aria-label="添加图片"><ImageOutlined sx={{ fontSize: 20 }} /></IconButton>
          <IconButton size="small" aria-label="智能增强"><AutoAwesomeOutlined sx={{ fontSize: 20 }} /></IconButton>
          {streaming ? (
            <Button sx={{ ml: 'auto', borderRadius: 2 }} variant="outlined" color="error" startIcon={<StopCircleOutlined />} onClick={onStop}>停止</Button>
          ) : (
            <Button sx={{ ml: 'auto', minWidth: 100, height: 40, borderRadius: 2, background: 'linear-gradient(135deg,#4e5be8,#5c44e7)' }} type="submit" variant="contained" startIcon={<SendOutlined />} disabled={disabled || !draft.trim()}>发送</Button>
          )}
        </Stack>
      </Box>
    </Box>
  );
}
