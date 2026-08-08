import AddOutlined from '@mui/icons-material/AddOutlined';
import FolderOutlined from '@mui/icons-material/FolderOutlined';
import { Box, Button, List, ListItemIcon, ListItemText, ListItemButton, Stack, TextField, Typography } from '@mui/material';
import { useState } from 'react';
import type { DirectoryNode } from '../types';

function TreeNodes({ nodes, level = 0, selectedId, onSelect }: {
  nodes: DirectoryNode[];
  level?: number;
  selectedId: string | null;
  onSelect: (id: string | null) => void;
}) {
  return (
    <List disablePadding>
      {nodes.map((node) => (
        <li key={node.id}>
          <ListItemButton
            sx={{ pl: 2 + level * 2 }}
            selected={selectedId === node.id}
            onClick={() => onSelect(node.id)}
          >
            <ListItemIcon sx={{ minWidth: 36 }}><FolderOutlined fontSize="small" /></ListItemIcon>
            <ListItemText primary={node.name} />
          </ListItemButton>
          {node.children.length > 0 && <TreeNodes nodes={node.children} level={level + 1} selectedId={selectedId} onSelect={onSelect} />}
        </li>
      ))}
    </List>
  );
}

export function KnowledgeTree({ nodes, selectedId, onSelect, onCreateDirectory }: {
  nodes: DirectoryNode[];
  selectedId: string | null;
  onSelect: (id: string | null) => void;
  onCreateDirectory: (name: string) => void;
}) {
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState('');

  function submit() {
    if (name.trim() === '') return;
    onCreateDirectory(name.trim());
    setName('');
    setCreating(false);
  }

  return (
    <Box sx={{ p: 1 }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" px={1} py={0.5}>
        <Typography variant="subtitle2" color="text.secondary">目录</Typography>
        <Button size="small" startIcon={<AddOutlined />} onClick={() => setCreating(true)}>新建目录</Button>
      </Stack>
      {creating && (
        <Stack direction="row" spacing={1} px={1} pb={1}>
          <TextField
            size="small"
            autoFocus
            label="目录名称"
            value={name}
            onChange={(event) => setName(event.target.value)}
            onKeyDown={(event) => { if (event.key === 'Enter') submit(); }}
          />
          <Button size="small" variant="contained" onClick={submit}>创建</Button>
        </Stack>
      )}
      <ListItemButton selected={selectedId === null} onClick={() => onSelect(null)}>
        <ListItemIcon sx={{ minWidth: 36 }}><FolderOutlined fontSize="small" /></ListItemIcon>
        <ListItemText primary="全部文档" />
      </ListItemButton>
      <TreeNodes nodes={nodes} selectedId={selectedId} onSelect={onSelect} />
    </Box>
  );
}
