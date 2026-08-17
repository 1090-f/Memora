import AddOutlined from '@mui/icons-material/AddOutlined';
import ArrowUpwardRounded from '@mui/icons-material/ArrowUpwardRounded';
import CheckRounded from '@mui/icons-material/CheckRounded';
import StopCircleOutlined from '@mui/icons-material/StopCircleOutlined';
import { Box, IconButton, ListItemIcon, ListItemText, Menu, MenuItem, Stack, TextField } from '@mui/material';
import { useState, type FormEvent, type KeyboardEvent, type MouseEvent } from 'react';

export function ChatComposer({ draft, disabled, streaming, knowledgeBases, selectedKnowledgeBaseId, onDraftChange, onKnowledgeBaseChange, onSend, onStop }: {
  draft: string;
  disabled: boolean;
  streaming: boolean;
  knowledgeBases: Array<{ id: string; name: string }>;
  selectedKnowledgeBaseId: string;
  onDraftChange: (value: string) => void;
  onKnowledgeBaseChange: (knowledgeBaseId: string) => void;
  onSend: () => void;
  onStop: () => void;
}) {
  const [knowledgeBaseMenuAnchor, setKnowledgeBaseMenuAnchor] = useState<HTMLElement | null>(null);
  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!disabled && draft.trim()) onSend();
  };
  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) {
      event.preventDefault();
      if (!disabled && draft.trim()) onSend();
    }
  };
  const openKnowledgeBaseMenu = (event: MouseEvent<HTMLElement>) => {
    setKnowledgeBaseMenuAnchor(event.currentTarget);
  };
  const selectKnowledgeBase = (knowledgeBaseId: string) => {
    setKnowledgeBaseMenuAnchor(null);
    onKnowledgeBaseChange(knowledgeBaseId);
  };

  return (
    <Box component="form" sx={{ width: '100%', px: { xs: 2, md: 3 }, pb: 2.2, bgcolor: '#fff' }} onSubmit={submit}>
      <Box sx={{ width: '100%', maxWidth: 980, mx: 'auto', border: '1px solid #e1e5eb', borderRadius: 3.5, px: 1.4, py: 1.15, bgcolor: '#fff', boxShadow: '0 8px 28px rgba(31,45,90,.09)', transition: 'border-color .2s, box-shadow .2s', '&:focus-within': { borderColor: '#9aabdf', boxShadow: '0 10px 32px rgba(31,45,90,.12)' } }}>
        <TextField
          placeholder="问点什么？使用 @ 可以提及哦~"
          value={draft}
          onChange={(event) => onDraftChange(event.target.value)}
          disabled={disabled}
          fullWidth
          multiline
          minRows={2}
          maxRows={5}
          onKeyDown={handleKeyDown}
          variant="standard"
          slotProps={{ input: { disableUnderline: true } }}
        />
        <Stack direction="row" alignItems="center" gap={0.7} sx={{ mt: 0.5 }}>
          <IconButton
            size="small"
            aria-label="选择知识库"
            aria-haspopup="menu"
            aria-expanded={Boolean(knowledgeBaseMenuAnchor)}
            onClick={openKnowledgeBaseMenu}
            disabled={disabled || streaming}
            sx={{ color: '#687384' }}
          >
            <AddOutlined />
          </IconButton>
          <Menu
            anchorEl={knowledgeBaseMenuAnchor}
            open={Boolean(knowledgeBaseMenuAnchor)}
            onClose={() => setKnowledgeBaseMenuAnchor(null)}
            slotProps={{ paper: { sx: { minWidth: 230, maxHeight: 320, mt: 0.8, borderRadius: 2.5, boxShadow: '0 12px 36px rgba(31,45,90,.16)' } } }}
          >
            {knowledgeBases.length === 0 && <MenuItem disabled>暂无可用知识库</MenuItem>}
            {knowledgeBases.map((knowledgeBase) => (
              <MenuItem key={knowledgeBase.id} selected={knowledgeBase.id === selectedKnowledgeBaseId} onClick={() => selectKnowledgeBase(knowledgeBase.id)}>
                <ListItemIcon>{knowledgeBase.id === selectedKnowledgeBaseId && <CheckRounded fontSize="small" />}</ListItemIcon>
                <ListItemText primary={knowledgeBase.name} />
              </MenuItem>
            ))}
          </Menu>
          {streaming ? (
            <IconButton sx={{ ml: 'auto', width: 40, height: 40 }} color="error" aria-label="停止生成" onClick={onStop}><StopCircleOutlined /></IconButton>
          ) : (
            <IconButton sx={{ ml: 'auto', width: 40, height: 40, bgcolor: '#9fc9f7', color: '#fff', '&:hover': { bgcolor: '#75afea' }, '&.Mui-disabled': { bgcolor: '#dce9f7', color: '#fff' } }} type="submit" aria-label="发送" disabled={disabled || !draft.trim()}><ArrowUpwardRounded /></IconButton>
          )}
        </Stack>
      </Box>
    </Box>
  );
}
