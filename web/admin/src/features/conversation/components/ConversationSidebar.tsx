import AddOutlined from '@mui/icons-material/AddOutlined';
import ArrowForwardIosOutlined from '@mui/icons-material/ArrowForwardIosOutlined';
import ChatBubbleOutlineOutlined from '@mui/icons-material/ChatBubbleOutlineOutlined';
import SearchOutlined from '@mui/icons-material/SearchOutlined';
import { Box, Button, InputAdornment, List, ListItemButton, ListItemIcon, ListItemText, Stack, TextField, Typography } from '@mui/material';
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
      <Button startIcon={<AddOutlined />} variant="contained" disabled={disabled} onClick={onCreate} sx={{ height: 43, borderRadius: 2, background: 'linear-gradient(135deg,#4b5bea,#5d45e8)', boxShadow: '0 7px 16px rgba(74,70,220,.2)' }}>新建会话</Button>
      <TextField
        size="small"
        placeholder="搜索会话"
        value={keyword}
        onChange={(event) => setKeyword(event.target.value)}
        slotProps={{ input: { startAdornment: <InputAdornment position="start"><SearchOutlined fontSize="small" /></InputAdornment> } }}
        sx={{ '& .MuiOutlinedInput-root': { height: 40, borderRadius: 2 } }}
      />
      <List disablePadding sx={{ overflow: 'auto', flexGrow: 1 }}>
        {filtered.map((item) => (
          <ListItemButton key={item.id} selected={item.id === selectedId} onClick={() => onSelect(item.id)} sx={{ minHeight: 76, mb: 0.8, borderRadius: 2.5, px: 1.5, alignItems: 'flex-start', '&.Mui-selected': { bgcolor: '#eff0ff' }, '&.Mui-selected:hover': { bgcolor: '#e9ebff' } }}>
            <ListItemIcon sx={{ minWidth: 30, mt: 0.2, color: item.id === selectedId ? '#4c5ce9' : '#78859d' }}><ChatBubbleOutlineOutlined sx={{ fontSize: 18 }} /></ListItemIcon>
            <ListItemText
              primary={item.title}
              secondary={new Date(item.updated_at || item.created_at).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}
              primaryTypographyProps={{ noWrap: true, sx: { color: '#25314c', fontSize: 13, fontWeight: item.id === selectedId ? 650 : 500 } }}
              secondaryTypographyProps={{ sx: { color: '#8994a8', fontSize: 11.5, mt: 0.8 } }}
            />
            <Box sx={{ width: 7, height: 7, borderRadius: '50%', bgcolor: item.id === selectedId ? '#2db756' : '#93a0b6', mt: 1.2 }} />
          </ListItemButton>
        ))}
      </List>
      {filtered.length === 0 && <Typography color="text.secondary" textAlign="center" py={4}>暂无会话</Typography>}
      <Button variant="outlined" endIcon={<ArrowForwardIosOutlined sx={{ fontSize: 13 }} />} sx={{ mt: 'auto', height: 42, borderRadius: 2, color: '#58657c', borderColor: '#dfe3eb' }}>查看全部会话</Button>
    </Stack>
  );
}
