import SendOutlined from '@mui/icons-material/SendOutlined';
import StopCircleOutlined from '@mui/icons-material/StopCircleOutlined';
import { Button, Stack, TextField } from '@mui/material';
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
    <Stack component="form" direction="row" spacing={1.5} p={2} borderTop="1px solid #e5e6ea" onSubmit={submit}>
      <TextField
        label="输入问题"
        value={draft}
        onChange={(event) => onDraftChange(event.target.value)}
        disabled={disabled}
        fullWidth
        multiline
        maxRows={5}
      />
      {streaming ? (
        <Button variant="outlined" color="error" startIcon={<StopCircleOutlined />} onClick={onStop}>停止</Button>
      ) : (
        <Button type="submit" variant="contained" startIcon={<SendOutlined />} disabled={disabled || !draft.trim()}>发送</Button>
      )}
    </Stack>
  );
}
