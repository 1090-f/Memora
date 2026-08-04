import { ConstsCrawlerSource } from '@/request';
import { Box } from '@mui/material';
import { useCallback, useMemo } from 'react';
import { Virtuoso } from 'react-virtuoso';
import { ListDataItem } from '..';
import { useGlobalQueue } from '../hooks/useGlobalQueue';
import BatchActionBar from './Action';
import ListRenderItem from './Item';

interface ListRenderProps {
  data: ListDataItem[];
  setData: React.Dispatch<React.SetStateAction<ListDataItem[]>>;
  checked: string[];
  setChecked: React.Dispatch<React.SetStateAction<string[]>>;
  parent_id: string | null;
  loading: boolean;
  type: ConstsCrawlerSource;
  isSupportSelect: boolean;
  queue: ReturnType<typeof useGlobalQueue>;
}

interface FlattenedItem {
  item: ListDataItem;
  depth: number;
}

const ListRender = ({
  data,
  checked,
  setChecked,
  loading,
  setData,
  type,
  isSupportSelect,
  parent_id,
  queue,
}: ListRenderProps) => {
  // 将树形数据展平为线性列表（只包含展开的节点）
  const flattenedData = useMemo(() => {
    const result: FlattenedItem[] = [];

    const flatten = (parentId: string | null, depth: number) => {
      const children = data.filter(item => item.parent_id === parentId);
      children.forEach(item => {
        result.push({ item, depth });
        // 如果是文件夹且展开，递归处理子节点
        if (!item.file && item.open && item.id) {
          flatten(item.id, depth + 1);
        }
      });
    };

    flatten(parent_id || '', 0);
    return result;
  }, [data, parent_id]);

  const getDescendantUuids = useCallback(
    (parentId: string): string[] => {
      const children = data.filter(item => item.parent_id === parentId);
      let uuids: string[] = [];

      children.forEach(child => {
        uuids.push(child.uuid);
        if (!child.file && child.id) {
          uuids = uuids.concat(getDescendantUuids(child.id));
        }
      });

      return uuids;
    },
    [data],
  );

  const handleSelectAllFolder = useCallback(
    (uuids: string[]) => {
      if (uuids.length === 0) return;
      setChecked(prev => {
        const set = new Set(prev);
        uuids.forEach(id => set.add(id));
        return Array.from(set);
      });
    },
    [setChecked],
  );

  const handleCancelSelectAllFolder = useCallback(
    (uuids: string[]) => {
      if (uuids.length === 0) return;
      setChecked(prev => prev.filter(id => !uuids.includes(id)));
    },
    [setChecked],
  );

  // 渲染虚拟列表项
  const itemContent = useCallback(
    (index: number, flattenedItem: FlattenedItem) => {
      const { item, depth } = flattenedItem;
      return (
        <ListRenderItem
          type={type}
          data={item}
          depth={depth}
          isSupportSelect={isSupportSelect}
          checked={checked.includes(item.uuid)}
          setData={setData}
          setChecked={setChecked}
          showSelectFolderAllBtn={
            !item.file &&
            !!item.folderReq &&
            (() => {
              const uuids = item.id ? getDescendantUuids(item.id) : [];
              const allUuids = item.folderReq ? [item.uuid, ...uuids] : uuids;
              if (allUuids.length === 0) return false;
              const selectedCount = allUuids.filter(id =>
                checked.includes(id),
              ).length;
              // 所有子项都没选中：只显示"全选文件夹"按钮
              if (selectedCount === 0) return true;
              if (selectedCount === allUuids.length) return false;
              return true;
            })()
          }
          showCancelSelectFolderAllBtn={
            !item.file &&
            !!item.folderReq &&
            (() => {
              const uuids = item.id ? getDescendantUuids(item.id) : [];
              const allUuids = item.folderReq ? [item.uuid, ...uuids] : uuids;
              if (allUuids.length === 0) return false;
              const selectedCount = allUuids.filter(id =>
                checked.includes(id),
              ).length;
              if (selectedCount === 0) return false;
              if (selectedCount === allUuids.length) return true;
              return true;
            })()
          }
          onSelectFolderAll={() => {
            if (!item.id) return;
            const uuids = getDescendantUuids(item.id);
            const allUuids = item.folderReq ? [item.uuid, ...uuids] : uuids;
            handleSelectAllFolder(allUuids);
          }}
          onCancelSelectFolderAll={() => {
            if (!item.id) return;
            const uuids = getDescendantUuids(item.id);
            const allUuids = item.folderReq ? [item.uuid, ...uuids] : uuids;
            handleCancelSelectAllFolder(allUuids);
          }}
        />
      );
    },
    [
      checked,
      setChecked,
      setData,
      isSupportSelect,
      type,
      getDescendantUuids,
      handleSelectAllFolder,
      handleCancelSelectAllFolder,
    ],
  );

  return (
    <Box
      sx={{
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: '10px',
        height: 'calc(100vh - 300px)',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      <BatchActionBar
        data={data}
        checked={checked}
        setChecked={setChecked}
        loading={loading}
        setData={setData}
        type={type}
        isSupportSelect={isSupportSelect}
        parent_id={parent_id}
        queue={queue}
      />
      <Box sx={{ flex: 1, overflow: 'hidden' }}>
        <Virtuoso
          data={flattenedData}
          totalCount={flattenedData.length}
          itemContent={itemContent}
          style={{ height: '100%' }}
        />
      </Box>
    </Box>
  );
};

export default ListRender;
