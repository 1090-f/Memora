'use client';

import React from 'react';
import { styled, Grid, alpha, Stack } from '@mui/material';
import { StyledTopicBox, StyledTopicTitle } from '../component/styledCommon';
import {
  useFadeInText,
  useCardFadeInAnimation,
} from '../hooks/useGsapAnimation';

interface BlockGridProps {
  mobile?: boolean;
  title?: string;
  items?: {
    name: string;
    url: string;
  }[];
}
const StyledBlockGridItem = styled(Stack)(({ theme }) => ({
  aspectRatio: '1 / 1',
  position: 'relative',
  border: `1px solid ${alpha(theme.palette.text.primary, 0.15)}`,
  borderRadius: '10px',
  padding: theme.spacing(1),
  boxShadow: `0px 5px 20px 0px ${alpha(theme.palette.text.primary, 0.06)}`,
  transition: 'all 0.2s ease',
  '&:hover': {
    color: theme.palette.primary.main,
    borderColor: theme.palette.primary.main,
    boxShadow: `0px 10px 20px 0px ${alpha(theme.palette.text.primary, 0.1)}`,
  },
  opacity: 0,
}));

export const StyledBlockGridItemImgBox = styled('div')(({ theme }) => ({
  flex: 1,
  overflow: 'hidden',
}));

export const StyledBlockGridItemImg = styled('img')(({ theme }) => ({
  width: '100%',
  height: '100%',
  objectFit: 'cover',
  borderRadius: '10px',
}));

const StyledBlockGridItemTitle = styled('div')(({ theme }) => ({
  position: 'absolute',
  bottom: '24px',
  left: '50%',
  maxWidth: 'calc(100% - 24px)',
  transform: 'translateX(-50%)',
  padding: theme.spacing(0.5, 1),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  flexShrink: 0,
  whiteSpace: 'nowrap',
  fontSize: 14,
  textAlign: 'center',
  fontWeight: 700,
  color: theme.palette.background.default,
  backgroundColor: alpha(theme.palette.text.primary, 0.5),
  borderRadius: '6px',
}));

// 单个卡片组件，带动画效果
const BlockGridItem: React.FC<{
  item: {
    name: string;
    url: string;
  };
  index: number;
}> = React.memo(({ item, index }) => {
  const cardRef = useCardFadeInAnimation(0.2 + index * 0.1, 0.1);
  return (
    <StyledBlockGridItem ref={cardRef as React.Ref<HTMLDivElement>} gap={2}>
      <StyledBlockGridItemImgBox>
        <StyledBlockGridItemImg src={item.url} />
      </StyledBlockGridItemImgBox>

      <StyledBlockGridItemTitle>{item.name}</StyledBlockGridItemTitle>
    </StyledBlockGridItem>
  );
});

const BlockGrid: React.FC<BlockGridProps> = React.memo(
  ({ title, items = [], mobile }) => {
    const size =
      typeof mobile === 'boolean'
        ? mobile
          ? 12
          : { xs: 12, md: 4 }
        : { xs: 12, md: 4 };

    // 添加标题淡入动画
    const titleRef = useFadeInText(0.2, 0.1);

    return (
      <StyledTopicBox>
        <StyledTopicTitle ref={titleRef}>{title}</StyledTopicTitle>
        <Grid container spacing={3} sx={{ width: '100%' }}>
          {items.map((item, index) => (
            <Grid size={size} key={index}>
              <BlockGridItem item={item} index={index} />
            </Grid>
          ))}
        </Grid>
      </StyledTopicBox>
    );
  },
);

export default BlockGrid;
