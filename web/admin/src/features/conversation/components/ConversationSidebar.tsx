import AddOutlined from '@mui/icons-material/AddOutlined';
import ChatOutlined from '@mui/icons-material/ChatOutlined';
import DeleteOutline from '@mui/icons-material/DeleteOutline';
import MoreVert from '@mui/icons-material/MoreVert';
import { Box, Button, IconButton, List, ListItemButton, ListItemIcon, ListItemText, Menu, MenuItem, Stack, Typography } from '@mui/material';
import { type MouseEvent, useState } from 'react';
import type { Conversation } from '../types';

export function ConversationSidebar({ conversations, selectedId, disabled, onSelect, onCreate, onDelete }: {
  conversations: Conversation[];
  selectedId?: string;
  disabled: boolean;
  onSelect: (id: string) => void;
  onCreate: () => void;
  onDelete: (id: string) => void;
}) {
  const [menuAnchor, setMenuAnchor] = useState<{ id: string; anchor: HTMLElement } | null>(null);

  const openMenu = (id: string, event: MouseEvent<HTMLElement>) => {
    event.stopPropagation();
    setMenuAnchor({ id, anchor: event.currentTarget });
  };

  const closeMenu = () => setMenuAnchor(null);

  const handleDelete = () => {
    if (menuAnchor) {
      onDelete(menuAnchor.id);
      closeMenu();
    }
  };

  return (
    <Stack spacing={1} height="100%">
      <Box px={2} pt={2} pb={0.5}>
        <Button startIcon={<AddOutlined />} variant="contained" fullWidth disabled={disabled} onClick={onCreate}>新建会话</Button>
      </Box>
      <List sx={{ flex: 1, overflow: 'auto', py: 0.5 }}>
        {conversations.map((item) => (
          <ListItemButton
            key={item.id}
            selected={item.id === selectedId}
            onClick={() => onSelect(item.id)}
            sx={{
              mx: 1,
              borderRadius: 1.5,
              py: 1,
              '&.Mui-selected': {
                bgcolor: 'primary.main',
                color: 'primary.contrastText',
                '&:hover': { bgcolor: 'primary.dark' },
                '& .MuiListItemIcon-root': { color: 'primary.contrastText' },
                '& .MuiListItemText-secondary': { color: 'primary.contrastText' },
                '& .menu-btn': { color: 'primary.contrastText' },
              },
              '&:not(.Mui-selected):hover .menu-btn': { opacity: 1 },
            }}
          >
            <ListItemIcon sx={{ minWidth: 40 }}>
              <ChatOutlined fontSize="small" />
            </ListItemIcon>
            <ListItemText
              primary={item.title}
              secondary={new Date(item.created_at).toLocaleDateString('zh-CN', {
                month: 'short',
                day: 'numeric',
                hour: '2-digit',
                minute: '2-digit',
              })}
              primaryTypographyProps={{
                noWrap: true,
                variant: 'body2',
                fontWeight: item.id === selectedId ? 600 : 400,
              }}
              secondaryTypographyProps={{ variant: 'caption' }}
              sx={{ mr: 0.5 }}
            />
            <IconButton
              className="menu-btn"
              size="small"
              onClick={(event) => openMenu(item.id, event)}
              sx={{ opacity: 0, transition: 'opacity 0.15s' }}
            >
              <MoreVert fontSize="small" />
            </IconButton>
          </ListItemButton>
        ))}
      </List>
      {conversations.length === 0 && (
        <Typography color="text.secondary" textAlign="center" py={4} variant="body2">暂无会话</Typography>
      )}

      <Menu
        anchorEl={menuAnchor?.anchor}
        open={!!menuAnchor}
        onClose={closeMenu}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
      >
        <MenuItem onClick={handleDelete} dense>
          <DeleteOutline fontSize="small" sx={{ mr: 1 }} />
          删除会话
        </MenuItem>
      </Menu>
    </Stack>
  );
}