import NoData from '@/assets/images/nodata.png';
import { DndContext } from '@dnd-kit/core';
import AccountCircleIcon from '@mui/icons-material/AccountCircle';
import { Box, IconButton, Menu, MenuItem, Stack } from '@mui/material';
import dayjs from 'dayjs';
import React, {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';

import {
  SortableTree,
  TreeItemComponentProps,
  TreeItems,
  TreeItemWrapper,
} from '@/components/TreeDragSortable';
import { ItemChangedReason } from '@/components/TreeDragSortable/types';
import { treeSx } from '@/constant/styles';
import { getApiProV1AuthGroupDetail } from '@/request/pro/AuthGroup';
import {
  GithubComChaitinPandaWikiProApiAuthV1AuthGroupTreeItem,
  ConstsSourceType,
} from '@/request/pro/types';
import { useAppSelector } from '@/store';
import { Modal, Table } from '@ctzhian/ui';
import { ColumnType } from '@ctzhian/ui/dist/Table';
import { IconGengduo, IconYonghuwenjianjia } from '@panda-wiki/icons';

type TreeNode = {
  id: string | number;
  name: string;
  level?: number;
  parentId?: string | number | null;
  auth_ids?: number[];
  children?: TreeNode[];
  isRoot?: boolean;
  count?: number;
  sync_id?: string;
};

export interface GroupTreeProps {
  data: GithubComChaitinPandaWikiProApiAuthV1AuthGroupTreeItem[];
  onDelete?: (id: number) => void;
  onClickMembers: (
    item: GithubComChaitinPandaWikiProApiAuthV1AuthGroupTreeItem,
  ) => void;
  onMove?: (args: {
    id: number;
    newParentId?: number;
    prev_id?: number;
    next_id?: number;
  }) => Promise<void>;
  onEdit?: (
    item: GithubComChaitinPandaWikiProApiAuthV1AuthGroupTreeItem,
    type: 'add' | 'edit',
  ) => void;
  sync?: boolean;
  onSync?: () => void;
  sourceType?: ConstsSourceType;
}

interface IContext {
  onClickMembers: (
    item: GithubComChaitinPandaWikiProApiAuthV1AuthGroupTreeItem,
  ) => void;
  handleMenuOpen: (
    event: React.MouseEvent<HTMLElement>,
    item: TreeNode,
  ) => void;
  sync: boolean;
  onSync?: () => void;
  sourceType?: ConstsSourceType;
}

const AppContext = createContext<IContext | null>(null);

const mapToTree = (
  list: GithubComChaitinPandaWikiProApiAuthV1AuthGroupTreeItem[],
  parentId: string | number | null = null,
): TreeNode[] => {
  return (list || []).map(it => ({
    id: it.id!,
    name: it.name || '',
    level: it.level,
    parentId: parentId ?? parentId,
    auth_ids: it.auth_ids,
    count: it.count,
    sync_id: it.sync_id,
    children: mapToTree(it.children || [], it.id!),
  }));
};

const TreeItem = React.forwardRef<
  HTMLDivElement,
  TreeItemComponentProps<TreeNode>
>((props, ref) => {
  const { item } = props;
  const context = useContext(AppContext);
  if (!context) throw new Error('TreeItem 必须在 AppContext.Provider 内部使用');
  const { onClickMembers, handleMenuOpen, sync, onSync, sourceType } = context;
  return (
    <Box
      sx={[
        treeSx(true),
        {
          px: 1,
          '&:hover': {
            '.dnd-sortable-tree_simple_handle': {
              opacity: 1,
            },
          },
        },
      ]}
    >
      <Stack direction='row' alignItems={'center'} gap={item.isRoot ? 2 : 0}>
        {!item.isRoot ? (
          <div
            {...props.handleProps}
            className={'dnd-sortable-tree_simple_handle'}
            style={sync ? { background: 'none' } : {}}
          />
        ) : (
          <div />
        )}
        <Box sx={{ flex: 1 }}>
          <TreeItemWrapper
            {...props}
            indentationWidth={23}
            disableCollapseOnItemClick={false}
            showDragHandle={false}
            ref={ref}
          >
            <Stack
              direction='row'
              alignItems={'center'}
              justifyContent='space-between'
              gap={2}
              flex={1}
              sx={{ pr: 2 }}
            >
              <Box sx={{ fontSize: 14 }}>
                <Box>{item.name}</Box>
              </Box>

              <Stack
                direction='row'
                alignItems={'center'}
                justifyContent='space-between'
                gap={1}
                sx={{ flexShrink: 1, width: 240 }}
              >
                {!item.isRoot ? (
                  <Box
                    sx={{
                      color: 'info.main',
                      cursor: 'pointer',
                      fontSize: 12,
                    }}
                    onClick={e => {
                      e.stopPropagation();
                      if (item && onClickMembers)
                        onClickMembers(
                          item as GithubComChaitinPandaWikiProApiAuthV1AuthGroupTreeItem,
                        );
                    }}
                  >
                    共 {item?.count || 0} 个成员
                  </Box>
                ) : (
                  <Box />
                )}

                {!sync && (
                  <Box
                    onClick={e => e.stopPropagation()}
                    onMouseDown={e => e.stopPropagation()}
                    onPointerDown={e => e.stopPropagation()}
                  >
                    <IconButton
                      size='small'
                      onClick={e => handleMenuOpen(e, item)}
                    >
                      <IconGengduo sx={{ fontSize: 16 }} />
                    </IconButton>
                  </Box>
                )}
                {sync &&
                  item.isRoot &&
                  (sourceType === ConstsSourceType.SourceTypeDingTalk ||
                    sourceType === ConstsSourceType.SourceTypeWeCom) && (
                    <Box
                      sx={{
                        fontSize: 14,
                        color: 'primary.main',
                        cursor: 'pointer',
                      }}
                      onClick={e => {
                        e.stopPropagation();
                        onSync?.();
                      }}
                    >
                      同步
                    </Box>
                  )}
              </Stack>
            </Stack>
          </TreeItemWrapper>
        </Box>
      </Stack>
    </Box>
  );
});

const GroupTree = ({
  data,
  onDelete,
  onMove,
  onEdit,
  sync = false,
  onSync,
  sourceType,
}: GroupTreeProps) => {
  const itemsData = useMemo(() => mapToTree(data), [data]);
  const { kbDetail } = useAppSelector(state => state.config);

  const itemsWithRoot = useMemo<TreeItems<TreeNode>>(() => {
    if (!itemsData) return itemsData;
    const attachRootParent = (children: TreeNode[]): TreeNode[] =>
      (children || []).map(c => ({ ...c, parentId: 'root' }));
    return [
      {
        id: 'root',
        name: sync ? '同步用户组' : '自定义用户组',
        parentId: null,
        isRoot: true,
        children: attachRootParent(itemsData),
      },
    ];
  }, [itemsData, kbDetail]);

  const [isModalOpen, setIsModalOpen] = useState(false);
  const handleModalClose = () => {
    setIsModalOpen(false);
    setMenuItem(null);
  };
  const [items, setItems] = useState<TreeItems<TreeNode>>(itemsWithRoot);
  const [menuAnchorEl, setMenuAnchorEl] = useState<null | HTMLElement>(null);
  const [menuPosition, setMenuPosition] = useState<{
    top: number;
    left: number;
  } | null>(null);
  const [menuItem, setMenuItem] = useState<TreeNode | null>(null);
  const isMenuOpen = Boolean(menuAnchorEl || menuPosition);
  const handleMenuOpen = (
    event: React.MouseEvent<HTMLElement>,
    item: TreeNode,
  ) => {
    event.stopPropagation();
    setMenuAnchorEl(event.currentTarget);
    setMenuPosition({ top: event.clientY, left: event.clientX });
    setMenuItem(item);
  };
  const handleMenuClose = (event?: React.SyntheticEvent) => {
    event?.stopPropagation?.();
    setMenuAnchorEl(null);
    setMenuPosition(null);
    setTimeout(() => {
      setMenuItem(null);
    }, 200);
  };
  useEffect(() => {
    setItems(itemsWithRoot);
  }, [itemsWithRoot]);

  const searchPreAndNext = (
    items: TreeItems<TreeNode>,
    parentId: string,
    id: string,
  ) => {
    const bfs = [...items];
    let parent;
    while (bfs.length > 0) {
      const current = bfs.shift();
      if (current?.id === parentId) {
        parent = current;
        break;
      }
      bfs.push(...(current?.children || []));
    }
    if (!parent) return { prevItem: null, nextItem: null };
    const index = parent.children?.findIndex(item => item.id === id);
    if (index === -1) return { prevItem: null, nextItem: null };
    return {
      prevItem: index! > 0 ? parent.children?.[index! - 1] : null,
      nextItem:
        index! < parent.children!.length - 1
          ? parent.children?.[index! + 1]
          : null,
    };
  };

  const onClickMembers = (
    item: GithubComChaitinPandaWikiProApiAuthV1AuthGroupTreeItem,
  ) => {
    setIsModalOpen(true);
    setMenuItem(item as TreeNode);
  };

  return (
    <AppContext.Provider
      value={{ onClickMembers, handleMenuOpen, sync, onSync, sourceType }}
    >
      <GroupModal
        open={isModalOpen}
        onCancel={handleModalClose}
        data={menuItem as TreeNode}
      />
      <DndContext>
        <SortableTree
          items={items.map(it => ({ ...it }))}
          disableSorting={sync}
          onItemsChanged={(newItems, reason: ItemChangedReason<TreeNode>) => {
            if (reason.type === 'dropped') {
              const { parentId, id } = reason.draggedItem;
              // 根节点禁止拖动；拖动到根节点视为无父级
              if (String(id) !== 'root') {
                const newParent =
                  parentId && String(parentId) !== 'root'
                    ? (parentId as number)
                    : undefined;
                const { prevItem, nextItem } = searchPreAndNext(
                  newItems,
                  parentId as string,
                  id as string,
                );
                onMove?.({
                  id: id as number,
                  newParentId: newParent,
                  prev_id: prevItem?.id as number,
                  next_id: nextItem?.id as number,
                }).finally(() => {
                  // 无论成功失败都刷新当前树状态
                });
              }
            }
            setItems(newItems);
          }}
          TreeItemComponent={TreeItem}
        />
        <ActionsMenu
          anchorEl={menuAnchorEl}
          anchorPosition={menuPosition || undefined}
          open={isMenuOpen}
          onClose={handleMenuClose}
          canEdit={!menuItem?.isRoot}
          onAdd={() => {
            onEdit?.(
              menuItem as GithubComChaitinPandaWikiProApiAuthV1AuthGroupTreeItem,
              'add',
            );
          }}
          onEdit={() => {
            onEdit?.(
              menuItem as GithubComChaitinPandaWikiProApiAuthV1AuthGroupTreeItem,
              'edit',
            );
          }}
          onDelete={() => {
            if (!menuItem) return;
            if (String(menuItem.id) === 'root') return;
            onDelete?.(Number(menuItem.id));
          }}
        />
      </DndContext>
    </AppContext.Provider>
  );
};

export default GroupTree;

export const ActionsMenu = ({
  anchorEl,
  open,
  onClose,
  canEdit,
  onAdd,
  onEdit,
  onDelete,
  anchorPosition,
}: {
  anchorEl: HTMLElement | null;
  open: boolean;
  onClose: (event?: React.SyntheticEvent) => void;
  canEdit: boolean;
  onAdd: () => void;
  onEdit: () => void;
  onDelete: () => void;
  anchorPosition?: { top: number; left: number };
}) => {
  return (
    <Menu
      anchorEl={anchorEl}
      anchorReference={anchorPosition ? 'anchorPosition' : 'anchorEl'}
      anchorPosition={anchorPosition}
      open={open}
      onClose={
        onClose as (
          event: object,
          reason: 'backdropClick' | 'escapeKeyDown',
        ) => void
      }
      anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
      transformOrigin={{ vertical: 'top', horizontal: 'right' }}
    >
      <MenuItem
        onClick={e => {
          e.stopPropagation();
          onClose(e);
          onAdd();
        }}
      >
        添加子分组
      </MenuItem>
      {canEdit && (
        <MenuItem
          onClick={e => {
            e.stopPropagation();
            onClose(e);
            onEdit();
          }}
        >
          编辑
        </MenuItem>
      )}
      {canEdit && (
        <MenuItem
          sx={{
            color: 'error.main',
          }}
          onClick={e => {
            e.stopPropagation();
            onClose(e);
            onDelete();
          }}
        >
          删除
        </MenuItem>
      )}
    </Menu>
  );
};

const GroupModal = ({
  open,
  onCancel,
  data,
}: {
  open: boolean;
  onCancel: () => void;
  data: TreeNode;
}) => {
  const { kb_id } = useAppSelector(state => state.config);
  const [groupList, setGroupList] = useState<any[]>([]);
  const columns: ColumnType<any>[] = [
    {
      title: '用户名',
      dataIndex: 'name',
      render: (text: string, record) => {
        return (
          <Stack direction={'row'} alignItems={'center'} gap={1}>
            {record.type === 'user' &&
              (record.avatar_url ? (
                <img
                  src={record.avatar_url}
                  width={16}
                  style={{ borderRadius: '50%' }}
                />
              ) : (
                <AccountCircleIcon
                  sx={{ fontSize: 16, color: 'text.secondary' }}
                />
              ))}
            {record.type === 'group' && (
              <IconYonghuwenjianjia sx={{ fontSize: 16, color: 'info.main' }} />
            )}
            {text}
          </Stack>
        );
      },
    },
    {
      title: 'created_at',
      dataIndex: 'created_at',
      render: (text: string, record) => {
        return record.type === 'user' ? (
          <Box sx={{ color: 'text.secondary' }}>
            {dayjs(text).fromNow()}加入，
            {dayjs(record.last_login_time).fromNow()}活跃
          </Box>
        ) : (
          <Box sx={{ color: 'text.secondary' }}>共 {record.count} 个成员</Box>
        );
      },
    },
  ];

  useEffect(() => {
    if (!open || !data) return;
    getApiProV1AuthGroupDetail({
      kb_id: kb_id,
      id: data.id as number,
    }).then(res => {
      const arr: any[] = [];
      res.auths?.forEach(it => {
        arr.push({
          ...it,
          type: 'user',
          name: it.username,
        });
      });
      res.children?.forEach(it => {
        arr.push({
          ...it,
          type: 'group',
        });
      });
      setGroupList(arr);
    });
  }, [open, data]);

  return (
    <Modal
      title={data?.name}
      width={800}
      open={open}
      showCancel={false}
      onCancel={onCancel}
      onOk={onCancel}
      okText='关闭'
    >
      <Table
        columns={columns}
        dataSource={groupList}
        showHeader={false}
        rowKey='id'
        size='small'
        sx={{
          '.MuiTableContainer-root': {
            maxHeight: 400,
            border: '1px dashed',
            borderColor: 'divider',
            borderRadius: '10px',
            borderBottom: 'none',
          },

          '.MuiTableCell-root': {
            px: '16px !important',
            height: 'auto !important',
          },
          '.MuiTableRow-root': {
            '&:hover': {
              '.MuiTableCell-root': {
                backgroundColor: 'transparent !important',
              },
            },
          },
        }}
        renderEmpty={
          <Stack alignItems={'center'}>
            <img src={NoData} width={124} />
            <Box>暂无数据</Box>
          </Stack>
        }
      />
    </Modal>
  );
};
