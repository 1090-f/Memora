'use client';

import React from 'react';
import { styled, Grid, Box, Button, alpha } from '@mui/material';
import {
  StyledTopicBox,
  StyledTopicTitle,
  StyledEllipsis,
} from '../component/styledCommon';
import { IconWenjianjia, IconWenjian, IconMulushu } from '@panda-wiki/icons';
import {
  useFadeInText,
  useCardFadeInAnimation,
} from '../hooks/useGsapAnimation';

interface NavDocProps {
  mobile?: boolean;
  title?: string;
  bgColor?: string;
  titleColor?: string;
  items?: {
    nav_id: string;
    nav_name: string;
    emoji?: string;
    list: {
      id: string;
      name: string;
      emoji?: string;
      type?: number;
      position?: number;
    }[];
  }[];
  basePath?: string;
}

const StyledNavDocItem = styled('div')(({ theme }) => ({
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'flex-start',
  justifyContent: 'space-between',
  gap: theme.spacing(2),
  padding: theme.spacing(3.5, 2.5, 2),
  borderRadius: '8px',
  boxShadow: `0px 5px 20px 0px ${alpha(theme.palette.text.primary, 0.06)}`,
  border: `1px solid ${alpha(theme.palette.text.primary, 0.1)}`,
  transition: 'all 0.2s ease',
  '&:hover': {
    transform: 'translateY(-5px)',
    boxShadow: '0px 10px 20px 0px rgba(0,0,5,0.2)',
    borderColor: theme.palette.primary.main,
  },
  width: '100%',
  opacity: 0,
}));

/** 顶部目录大图标区域 */
const StyledNavDocItemIcon = styled('div')(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  width: 30,
  height: 30,
  borderRadius: '4px',
  backgroundColor: alpha(theme.palette.primary.main, 0.1),
  flexShrink: 0,
}));

/** 目录标题行 */
const StyledNavDocItemTitle = styled('h3')(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  color: theme.palette.text.primary,
  gap: theme.spacing(1),
  fontSize: 20,
  fontWeight: 700,
  width: '100%',
  margin: 0,
}));

/** 子文件/文件夹列表容器 */
const StyledNavDocItemFiles = styled('div')(({ theme }) => ({
  display: 'flex',
  flex: '1 0 auto',
  flexDirection: 'column',
  justifyContent: 'flex-start',
  gap: theme.spacing(2),
  marginLeft: theme.spacing(0.5),
  fontSize: 14,
  fontWeight: 400,
  height: 129,
  width: '100%',
  lineHeight: 1.5,
}));

/** 单条子节点链接 */
const StyledNavDocItemFile = styled('a')(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  gap: theme.spacing(1),
  width: '100%',
  cursor: 'pointer',
  color: '#717572',
  '&:hover': {
    color: theme.palette.primary.main,
  },
}));

// 单个卡片组件，带动画效果
const NavDocItem: React.FC<{
  item: NonNullable<NavDocProps['items']>[number];
  index: number;
  basePath: string;
  size: any;
}> = React.memo(({ item, index, basePath, size }) => {
  const cardRef = useCardFadeInAnimation(0.2 + index * 0.1, 0.1);

  return (
    <Grid size={size}>
      <StyledNavDocItem ref={cardRef as React.Ref<HTMLDivElement>}>
        {/* 图标 + 目录名称（同一行） */}
        <StyledNavDocItemTitle>
          <StyledNavDocItemIcon>
            <IconMulushu sx={{ fontSize: 20, color: 'primary.main' }} />
          </StyledNavDocItemIcon>
          <StyledEllipsis>{item.nav_name}</StyledEllipsis>
        </StyledNavDocItemTitle>

        {/* 下级文件/文件夹列表（最多展示 4 条） */}
        <StyledNavDocItemFiles>
          {item.list?.slice(0, 4).map(it => (
            <StyledNavDocItemFile
              key={it.id}
              href={`${basePath}/node/${it.id}`}
              target='_blank'
            >
              {it.emoji ? (
                <Box sx={{ fontSize: 14, lineHeight: 1, flexShrink: 0 }}>
                  {it.emoji}
                </Box>
              ) : it.type === 1 ? (
                <IconWenjianjia sx={{ fontSize: 14, flexShrink: 0 }} />
              ) : (
                <IconWenjian sx={{ fontSize: 14, flexShrink: 0 }} />
              )}
              <StyledEllipsis>{it.name}</StyledEllipsis>
            </StyledNavDocItemFile>
          ))}
        </StyledNavDocItemFiles>

        {/* 查看更多按钮，跳转到目录页面 */}
        <Button
          href={item.list?.[0]?.id ? `${basePath}/node/${item.list[0].id}` : ''}
          target='_blank'
          sx={{ gap: 1, alignSelf: 'flex-end' }}
          variant='text'
          color='primary'
        >
          查看更多
        </Button>
      </StyledNavDocItem>
    </Grid>
  );
});

NavDocItem.displayName = 'NavDocItem';

const NavDoc: React.FC<NavDocProps> = React.memo(
  ({ title, items = [], mobile, basePath = '' }) => {
    const size =
      typeof mobile === 'boolean' ? (mobile ? 12 : 4) : { xs: 12, md: 4 };

    // 标题淡入动画
    const titleRef = useFadeInText(0.2, 0.1);

    return (
      <StyledTopicBox>
        <StyledTopicTitle ref={titleRef}>{title}</StyledTopicTitle>
        <Grid container spacing={3} sx={{ width: '100%' }}>
          {items.map((item, index) => (
            <NavDocItem
              key={item.nav_id || index}
              item={item}
              index={index}
              basePath={basePath}
              size={size}
            />
          ))}
        </Grid>
      </StyledTopicBox>
    );
  },
);

NavDoc.displayName = 'NavDoc';

export default NavDoc;
