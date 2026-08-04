import { ITreeItem } from '@/api';
import Nodata from '@/assets/images/nodata.png';
import Card from '@/components/Card';
import DragTree from '@/components/Drag/DragTree';
import { getApiV1NodeListGroupNav } from '@/request/Node';
import {
  DomainNodeListItemResp,
  DomainNodeType,
  GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp,
} from '@/request/types';
import { useAppSelector } from '@/store';
import { convertToTree } from '@/utils/drag';
import { Modal } from '@ctzhian/ui';
import { Box, Checkbox, IconButton, Skeleton, Stack } from '@mui/material';
import { IconXiajiantou } from '@panda-wiki/icons';
import { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react';

/** 兼容不同版本 API 返回结构 */
function normalizeNavGroupResponse(
  res: unknown,
): GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[] {
  if (Array.isArray(res))
    return res as GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[];
  if (res && typeof res === 'object') {
    const obj = res as Record<string, unknown>;
    for (const key of ['list', 'data', 'groups', 'items']) {
      if (Array.isArray(obj[key]))
        return obj[
          key
        ] as GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[];
    }
  }
  return [];
}

/** 从单个导航分组中提取节点列表 */
function getNavNodeList(
  nav:
    | GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp
    | Record<string, unknown>,
): DomainNodeListItemResp[] {
  const n = nav as Record<string, unknown>;
  return (
    (n['list'] as DomainNodeListItemResp[] | undefined) ||
    (n['nodes'] as DomainNodeListItemResp[] | undefined) ||
    (n['items'] as DomainNodeListItemResp[] | undefined) ||
    []
  );
}

// ─── 单个导航分组 Card（memo 包裹，避免无关 selectedIds 变化导致的重渲染）───

interface NavGroupCardProps {
  navId: string;
  navName: string;
  navNodes: DomainNodeListItemResp[];
  /** 已经转好的树形结构（静态，不随选择变化） */
  navTreeList: ITreeItem[];
  /** 可选节点（已过滤 disabled）的 id 集合 */
  selectableIds: string[];
  /** 当前组内可选节点总数 */
  navTotal: number;
  /** 当前已选中 id 集合（Set，O(1) 查找） */
  selectedSet: Set<string>;
  isExpanded: boolean;
  readOnly: boolean;
  disabled?: (value: ITreeItem) => boolean;
  onToggleExpand: (navId: string) => void;
  onSelectChange: (ids: string[]) => void;
  onGroupSelectAll: (selectableIds: string[], allSelected: boolean) => void;
  refresh: () => void;
}

const NavGroupCard = memo(
  ({
    navId,
    navName,
    navNodes,
    navTreeList,
    selectableIds,
    navTotal,
    selectedSet,
    isExpanded,
    readOnly,
    disabled,
    onToggleExpand,
    onSelectChange,
    onGroupSelectAll,
    refresh,
  }: NavGroupCardProps) => {
    // 当前组已选数量：只对可选节点计数
    const navSelectedCount = useMemo(
      () => selectableIds.filter(id => selectedSet.has(id)).length,
      [selectableIds, selectedSet],
    );

    // DragTree 需要的是数组形式的已选 id
    const selectedArr = useMemo(() => [...selectedSet], [selectedSet]);

    return (
      <Card
        key={navId}
        sx={{ bgcolor: 'background.paper3', overflow: 'hidden' }}
      >
        {/* 分组标题行：折叠箭头 + 名称 + 计数 + 全选 */}
        <Stack
          direction='row'
          component='label'
          alignItems='center'
          sx={{ py: 1, px: 1.5, cursor: 'pointer', fontSize: 14 }}
        >
          <IconButton
            size='small'
            onClick={e => {
              e.preventDefault();
              onToggleExpand(navId);
            }}
            sx={{ p: 0.25, mr: 0.5 }}
          >
            <IconXiajiantou
              sx={{
                fontSize: 16,
                color: 'text.disabled',
                transform: isExpanded ? 'none' : 'rotate(-90deg)',
                transition: 'transform 0.2s',
              }}
            />
          </IconButton>
          <Box sx={{ flex: 1 }}>
            {navName}
            <Box
              component='span'
              sx={{ color: 'text.tertiary', fontSize: 12, pl: 1 }}
            >
              共 {navTotal} 个
              {navSelectedCount > 0 ? `，已选中 ${navSelectedCount} 个` : ''}
            </Box>
          </Box>
          <Stack direction='row' alignItems='center'>
            <Box sx={{ color: 'text.tertiary', fontSize: 12 }}>全选</Box>
            <Checkbox
              size='small'
              sx={{
                p: 0,
                color: 'text.disabled',
                width: '35px',
                height: '35px',
              }}
              checked={navTotal > 0 && navSelectedCount === navTotal}
              onChange={() =>
                onGroupSelectAll(selectableIds, navSelectedCount === navTotal)
              }
            />
          </Stack>
        </Stack>

        {/* 折叠展开区域 */}
        {isExpanded && (
          <Stack gap={0.25} sx={{ fontSize: 14, px: 2, pb: 1 }}>
            <DragTree
              ui='select'
              readOnly={readOnly}
              selected={selectedArr}
              data={navTreeList}
              refresh={refresh}
              onSelectChange={onSelectChange}
              disabled={disabled}
              relativeSelect={false}
            />
          </Stack>
        )}
      </Card>
    );
  },
);

NavGroupCard.displayName = 'NavGroupCard';

// ─── 主组件 ───────────────────────────────────────────────────────────────────

interface AddRecommendContentProps {
  open: boolean;
  selected: string[];
  onChange: (value: string[]) => void;
  onClose: () => void;
  disabled?: (value: ITreeItem) => boolean;
  readOnly?: boolean;
  nodeType?: DomainNodeType;
}

const AddRecommendContent = ({
  open,
  selected,
  onChange,
  onClose,
  disabled,
  readOnly = true,
}: AddRecommendContentProps) => {
  const [navList, setNavList] = useState<
    GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[]
  >([]);
  const [loading, setLoading] = useState(false);
  const { kb_id } = useAppSelector(state => state.config);
  const [selectedIds, setSelectedIds] = useState<string[]>(selected);
  const [expandedNavIds, setExpandedNavIds] = useState<Set<string>>(new Set());

  // 用 Set 存储已选 id，O(1) 查找，避免每次 includes 都 O(n)
  const selectedSet = useMemo(() => new Set(selectedIds), [selectedIds]);

  const getData = useCallback(() => {
    setLoading(true);
    getApiV1NodeListGroupNav({ kb_id, status: 'released' })
      .then(res => {
        const navData = normalizeNavGroupResponse(res);
        setNavList(navData);
        // 默认全部展开
        setExpandedNavIds(
          new Set(navData.map((nav, idx) => nav.nav_id || `nav-${idx}`)),
        );
      })
      .finally(() => {
        setLoading(false);
      });
  }, [kb_id]);

  useEffect(() => {
    setSelectedIds(selected);
  }, [selected]);

  useEffect(() => {
    if (open && kb_id) getData();
  }, [open, kb_id, getData]);

  // 对 navList 的静态派生数据做 memo，避免每次渲染重算
  const navGroups = useMemo(() => {
    return navList
      .map((nav, idx) => {
        const navId = nav.nav_id || `nav-${idx}`;
        const navNodes = getNavNodeList(nav);
        if (navNodes.length === 0) return null;

        // convertToTree 开销较大，只在 navList 变化时执行
        const navTreeList = convertToTree(navNodes);

        // 过滤掉 disabled 返回 true 的节点（不可选），不纳入总数和全选范围
        const selectableNodes = disabled
          ? navNodes.filter(n => !disabled(n as ITreeItem))
          : navNodes;
        const selectableIds = selectableNodes.map(n => n.id!);

        return {
          navId,
          navName: nav.nav_name || '未分类',
          navNodes,
          navTreeList,
          selectableIds,
          navTotal: selectableIds.length,
        };
      })
      .filter(Boolean) as {
      navId: string;
      navName: string;
      navNodes: DomainNodeListItemResp[];
      navTreeList: ITreeItem[];
      selectableIds: string[];
      navTotal: number;
    }[];
  }, [navList, disabled]);

  const isEmpty = !loading && navGroups.length === 0;

  // 稳定的回调引用，避免子组件不必要的重渲染
  const handleToggleExpand = useCallback((navId: string) => {
    setExpandedNavIds(prev => {
      const next = new Set(prev);
      if (next.has(navId)) next.delete(navId);
      else next.add(navId);
      return next;
    });
  }, []);

  const handleSelectChange = useCallback((ids: string[]) => {
    setSelectedIds(ids);
  }, []);

  // 全选/取消全选：只操作该分组内的可选节点
  const handleGroupSelectAll = useCallback(
    (selectableIds: string[], allSelected: boolean) => {
      if (allSelected) {
        setSelectedIds(prev => prev.filter(id => !selectableIds.includes(id)));
      } else {
        setSelectedIds(prev => {
          const added = new Set(prev);
          selectableIds.forEach(id => added.add(id));
          return [...added];
        });
      }
    },
    [],
  );

  return (
    <Modal
      title='添加卡片'
      open={open}
      onOk={() => {
        onChange(selectedIds);
        onClose();
      }}
      onCancel={onClose}
    >
      {loading ? (
        <Stack gap={2}>
          {new Array(10).fill(0).map((_, index) => (
            <Skeleton variant='text' height={20} key={index} />
          ))}
        </Stack>
      ) : isEmpty ? (
        <Stack alignItems={'center'} justifyContent={'center'}>
          <img src={Nodata} alt='empty' style={{ width: 100, height: 100 }} />
          <Box
            sx={{
              fontSize: 12,
              lineHeight: '20px',
              color: 'text.tertiary',
              mt: 1,
            }}
          >
            暂无数据，前往文档页面创建并发布文档
          </Box>
        </Stack>
      ) : (
        <Stack gap={1}>
          {navGroups.map(group => (
            <NavGroupCard
              key={group.navId}
              {...group}
              selectedSet={selectedSet}
              isExpanded={expandedNavIds.has(group.navId)}
              readOnly={readOnly}
              disabled={disabled}
              onToggleExpand={handleToggleExpand}
              onSelectChange={handleSelectChange}
              onGroupSelectAll={handleGroupSelectAll}
              refresh={getData}
            />
          ))}
        </Stack>
      )}
    </Modal>
  );
};

export default AddRecommendContent;
