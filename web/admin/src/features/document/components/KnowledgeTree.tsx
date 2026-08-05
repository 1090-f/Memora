import FolderOutlined from '@mui/icons-material/FolderOutlined';
import { List, ListItemIcon, ListItemText, ListItemButton } from '@mui/material';
import type { DirectoryNode } from '../types';

function TreeNodes({ nodes, level = 0 }: { nodes: DirectoryNode[]; level?: number }) {
  return (
    <List disablePadding>
      {nodes.map((node) => (
        <li key={node.id}>
          <ListItemButton sx={{ pl: 2 + level * 2 }}>
            <ListItemIcon sx={{ minWidth: 36 }}><FolderOutlined fontSize="small" /></ListItemIcon>
            <ListItemText primary={node.name} />
          </ListItemButton>
          {node.children.length > 0 && <TreeNodes nodes={node.children} level={level + 1} />}
        </li>
      ))}
    </List>
  );
}

export function KnowledgeTree({ nodes }: { nodes: DirectoryNode[] }) {
  return <TreeNodes nodes={nodes} />;
}
