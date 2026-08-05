import AddOutlined from '@mui/icons-material/AddOutlined';
import SearchOutlined from '@mui/icons-material/SearchOutlined';
import { Button, InputAdornment, List, ListItemButton, ListItemText, Stack, TextField, Typography } from '@mui/material';
import { useMemo, useState } from 'react';
import type { Conversation } from '../types';

export function ConversationSidebar({ conversations, selectedId, disabled, onSelect, onCreate }: {
  conversations: Conversation[];
  selectedId?: string;
  disabled: boolean;
  onSelect: (id: string) => void;
  onCreate: () => void;
}) {
  const [keyword, setKeyword] = useState('');
  const filtered = useMemo(() => conversations.filter((item) => item.title.toLowerCase().includes(keyword.toLowerCase())), [conversations, keyword]);
  return (
    <Stack spacing={1.5} p={2} height="100%">
      <Button startIcon={<AddOutlined />} variant="contained" disabled={disabled} onClick={onCreate}>新建会话</Button>
      <TextField
        size="small"
        label="搜索会话"
        value={keyword}
        onChange={(event) => setKeyword(event.target.value)}
        slotProps={{ input: { startAdornment: <InputAdornment position="start"><SearchOutlined fontSize="small" /></InputAdornment> } }}
      />
      <List disablePadding sx={{ overflow: 'auto' }}>
        {filtered.map((item) => (
          <ListItemButton key={item.id} selected={item.id === selectedId} onClick={() => onSelect(item.id)}>
            <ListItemText primary={item.title} secondary={new Date(item.created_at).toLocaleDateString()} />
          </ListItemButton>
        ))}
      </List>
      {filtered.length === 0 && <Typography color="text.secondary" textAlign="center" py={4}>暂无会话</Typography>}
    </Stack>
  );
}
