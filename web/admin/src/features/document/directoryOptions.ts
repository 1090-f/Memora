import type { DirectoryNode } from './types';

export function flattenDirectories(nodes: DirectoryNode[], depth = 0): Array<{ id: string; name: string; depth: number }> {
  // MUI Select 不直接支持树形选项，将目录深度保留下来用于缩进展示。
  return nodes.flatMap((node) => [
    { id: node.id, name: node.name, depth },
    ...flattenDirectories(node.children, depth + 1),
  ]);
}
