'use client';

import React from 'react';
import { styled, Grid, alpha, Stack } from '@mui/material';
import { StyledTopicBox, StyledTopicTitle } from '../component/styledCommon';
import {
  useFadeInText,
  useCardFadeInAnimation,
} from '../hooks/useGsapAnimation';
import { IconTips } from '@panda-wiki/icons';

interface FeatureProps {
  mobile?: boolean;
  title?: string;
  items?: {
    name: string;
    desc: string;
  }[];
}
const StyledFeatureItem = styled(Stack)(({ theme }) => ({
  border: `1px solid ${alpha(theme.palette.text.primary, 0.15)}`,
  borderRadius: '10px',
  padding: theme.spacing(2.5),
  boxShadow: `0px 5px 20px 0px ${alpha(theme.palette.text.primary, 0.06)}`,
  transition: 'all 0.2s ease',
  '&:hover': {
    color: theme.palette.primary.main,
    borderColor: theme.palette.primary.main,
    boxShadow: `0px 10px 20px 0px ${alpha(theme.palette.text.primary, 0.1)}`,
  },
  opacity: 0,
}));

export const StyledFeatureItemIcon = styled('div')(({ theme }) => ({
  width: 60,
  height: 60,
  borderRadius: '50%',
  backgroundColor: alpha(theme.palette.primary.main, 0.06),
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  flex: '0 0 60px',
}));

const StyledFeatureItemTitle = styled('span')(({ theme }) => ({
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
  fontSize: 20,
  fontWeight: 700,
  color: theme.palette.text.primary,
}));

const StyledFeatureItemSummary = styled('div')(({ theme }) => ({
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  display: '-webkit-box',
  WebkitLineClamp: 3,
  WebkitBoxOrient: 'vertical',
  height: 60,
  fontSize: 14,
  fontWeight: 400,
  color: alpha(theme.palette.text.primary, 0.5),
}));

// 单个卡片组件，带动画效果
const FeatureItem: React.FC<{
  item: {
    name: string;
    desc: string;
  };
  index: number;
}> = React.memo(({ item, index }) => {
  const cardRef = useCardFadeInAnimation(0.2 + index * 0.1, 0.1);
  return (
    <StyledFeatureItem
      ref={cardRef as React.Ref<HTMLDivElement>}
      gap={2.5}
      direction='row'
      alignItems='flex-start'
    >
      <StyledFeatureItemIcon>
        <IconTips sx={{ color: 'primary.main', fontSize: 24 }} />
      </StyledFeatureItemIcon>
      <Stack gap={1} sx={{ minWidth: 0 }}>
        <StyledFeatureItemTitle>{item.name}</StyledFeatureItemTitle>
        <StyledFeatureItemSummary>{item.desc}</StyledFeatureItemSummary>
      </Stack>
    </StyledFeatureItem>
  );
});

const Feature: React.FC<FeatureProps> = React.memo(
  ({ title, items = [], mobile }) => {
    const size =
      typeof mobile === 'boolean'
        ? mobile
          ? 12
          : { xs: 12, md: 6 }
        : { xs: 12, md: 6 };

    // 添加标题淡入动画
    const titleRef = useFadeInText(0.2, 0.1);

    return (
      <StyledTopicBox>
        <StyledTopicTitle ref={titleRef}>{title}</StyledTopicTitle>
        <Grid container spacing={3} sx={{ width: '100%' }}>
          {items.map((item, index) => (
            <Grid size={size} key={index}>
              <FeatureItem item={item} index={index} />
            </Grid>
          ))}
        </Grid>
      </StyledTopicBox>
    );
  },
);

export default Feature;
