import { useCallback, useEffect, useMemo, useState } from 'react';
import { ITreeItem } from '@/api';
import Nodata from '@/assets/images/nodata.png';
import Card from '@/components/Card';
import DragTree from '@/components/Drag/DragTree';
import { getApiV1NodeListGroupNav } from '@/request/Node';
import {
  DomainNodeListItemResp,
  GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp,
} from '@/request/types';
import { useAppSelector } from '@/store';
import { convertToTree } from '@/utils/drag';
import { Modal } from '@ctzhian/ui';
import { Box, Checkbox, IconButton, Skeleton, Stack } from '@mui/material';
import { IconXiajiantou } from '@panda-wiki/icons';
import { memo } from 'react';

interface AddNavContentProps {
  open: boolean;
  selected: string[];
  onChange: (ids: string[]) => void;
  onClose: () => void;
}

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

// ─── 单个导航分组 Card ──────────────────────────────────────────────────────────

interface NavGroupCardProps {
  navId: string;
  navName: string;
  navTreeList: ITreeItem[];
  isSelected: boolean;
  isExpanded: boolean;
  onToggleExpand: (navId: string) => void;
  onToggleSelect: (navId: string) => void;
  refresh: () => void;
}

const NavGroupCard = memo(
  ({
    navId,
    navName,
    navTreeList,
    isSelected,
    isExpanded,
    onToggleExpand,
    onToggleSelect,
    refresh,
  }: NavGroupCardProps) => {
    return (
      <Card sx={{ bgcolor: 'background.paper3', overflow: 'hidden' }}>
        <Stack
          direction='row'
          alignItems='center'
          sx={{ py: 1, px: 1.5, cursor: 'pointer', fontSize: 14 }}
        >
          <Checkbox
            size='small'
            checked={isSelected}
            onChange={() => onToggleSelect(navId)}
            sx={{ p: 0, mr: 0.5 }}
          />
          <IconButton
            size='small'
            onClick={() => onToggleExpand(navId)}
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
          <Box
            sx={{ flex: 1, cursor: 'pointer' }}
            onClick={() => onToggleExpand(navId)}
          >
            {navName}
          </Box>
        </Stack>

        {isExpanded && (
          <Stack gap={0.25} sx={{ fontSize: 14, px: 2, pb: 1 }}>
            <DragTree
              ui='select'
              readOnly
              selected={[]}
              data={navTreeList}
              refresh={refresh}
              disabled={() => true}
            />
          </Stack>
        )}
      </Card>
    );
  },
);

NavGroupCard.displayName = 'NavGroupCard';

// ─── 主组件 ───────────────────────────────────────────────────────────────────

const AddNavContent = ({
  open,
  selected,
  onChange,
  onClose,
}: AddNavContentProps) => {
  const { kb_id } = useAppSelector(state => state.config);
  const [loading, setLoading] = useState(false);
  const [navList, setNavList] = useState<
    GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[]
  >([]);
  const [selectedIds, setSelectedIds] = useState<string[]>(selected);
  const [expandedNavIds, setExpandedNavIds] = useState<Set<string>>(new Set());

  const selectedSet = useMemo(() => new Set(selectedIds), [selectedIds]);

  const getData = useCallback(() => {
    setLoading(true);
    getApiV1NodeListGroupNav({ kb_id, status: 'released' })
      .then(res => {
        const navData = normalizeNavGroupResponse(res);
        setNavList(navData);
        // setExpandedNavIds(
        //   new Set(navData.map((nav, idx) => nav.nav_id || `nav-${idx}`)),
        // );
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

  const navGroups = useMemo(() => {
    return navList
      .filter(nav => nav.list && nav.list.length > 0)
      .map(nav => {
        const navId = nav.nav_id!;
        const navNodes = getNavNodeList(nav);
        const navTreeList = navNodes.length > 0 ? convertToTree(navNodes) : [];
        return {
          navId,
          id: navId,
          name: nav.nav_name || '未分类',
          navName: nav.nav_name || '未分类',
          navNodes,
          navTreeList,
        };
      });
  }, [navList]);

  const isEmpty = !loading && navGroups.length === 0;

  const handleToggleExpand = useCallback((navId: string) => {
    setExpandedNavIds(prev => {
      const next = new Set(prev);
      if (next.has(navId)) next.delete(navId);
      else next.add(navId);
      return next;
    });
  }, []);

  const handleToggleSelect = useCallback((navId: string) => {
    setSelectedIds(prev =>
      prev.includes(navId) ? prev.filter(id => id !== navId) : [...prev, navId],
    );
  }, []);

  const handleConfirm = () => {
    onChange(selectedIds);
    onClose();
  };

  return (
    <Modal title='选择目录' open={open} onOk={handleConfirm} onCancel={onClose}>
      {loading ? (
        <Stack gap={2}>
          {new Array(5).fill(0).map((_, index) => (
            <Skeleton variant='text' height={36} key={index} />
          ))}
        </Stack>
      ) : isEmpty ? (
        <Stack alignItems='center' justifyContent='center'>
          <img src={Nodata} alt='empty' style={{ width: 100, height: 100 }} />
          <Box
            sx={{
              fontSize: 12,
              lineHeight: '20px',
              color: 'text.tertiary',
              mt: 1,
            }}
          >
            暂无目录数据
          </Box>
        </Stack>
      ) : (
        <Stack gap={1}>
          {navGroups.map(group => (
            <NavGroupCard
              key={group.navId}
              {...group}
              isSelected={selectedSet.has(group.navId)}
              isExpanded={expandedNavIds.has(group.navId)}
              onToggleExpand={handleToggleExpand}
              onToggleSelect={handleToggleSelect}
              refresh={getData}
            />
          ))}
        </Stack>
      )}
    </Modal>
  );
};

export default AddNavContent;
