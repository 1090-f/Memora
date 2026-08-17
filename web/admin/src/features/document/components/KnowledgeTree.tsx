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
            sx={{
              mx: 1,
              mb: 0.4,
              flex: '0 0 auto',
              pl: 2 + level * 2,
              minHeight: 43,
              borderRadius: 2,
              color: '#56637d',
              '&.Mui-selected': { bgcolor: '#eef0ff', color: '#4058e9' },
              '&.Mui-selected:hover': { bgcolor: '#e8ebff' },
            }}
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

export function KnowledgeTree({ nodes, selectedId, onSelect, onCreateDirectory, totalDocuments = 0 }: {
  nodes: DirectoryNode[];
  selectedId: string | null;
  onSelect: (id: string | null) => void;
  onCreateDirectory: (name: string, parentId?: string) => void;
  totalDocuments?: number;
}) {
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState('');

  function submit() {
    if (name.trim() === '') return;
    // 选中目录时创建其子目录；选中“全部文档”时创建根目录。
    onCreateDirectory(name.trim(), selectedId ?? undefined);
    setName('');
    setCreating(false);
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%', minHeight: 466 }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" px={1.5} py={1.35}>
        <Typography sx={{ color: '#27334e', fontSize: 13, fontWeight: 650 }}>目录</Typography>
        <Button size="small" startIcon={<AddOutlined />} onClick={() => setCreating(true)} sx={{ fontSize: 11.5 }}>{selectedId ? '新建子目录' : '新建目录'}</Button>
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
          <Button size="small" onClick={() => { setCreating(false); setName(''); }}>取消</Button>
        </Stack>
      )}
      <ListItemButton selected={selectedId === null} onClick={() => onSelect(null)} sx={{ mx: 1, mb: 0.4, minHeight: 43, flex: '0 0 auto', borderRadius: 2, color: '#56637d', '&.Mui-selected': { bgcolor: '#eef0ff', color: '#4058e9' }, '&.Mui-selected:hover': { bgcolor: '#e8ebff' } }}>
        <ListItemIcon sx={{ minWidth: 36 }}><FolderOutlined fontSize="small" /></ListItemIcon>
        <ListItemText primary="全部文档" />
        <Typography sx={{ color: '#8490a6', fontSize: 11 }}>{totalDocuments}</Typography>
      </ListItemButton>
      <TreeNodes nodes={nodes} selectedId={selectedId} onSelect={onSelect} />
      <Stack direction="row" alignItems="center" sx={{ mt: 'auto', px: 1.5, py: 1.2, borderTop: '1px solid #e8ebf1' }}>
        <Typography sx={{ color: '#66728b', fontSize: 12, flexGrow: 1 }}>共 {nodes.length} 个目录</Typography>
      </Stack>
    </Box>
  );
}
