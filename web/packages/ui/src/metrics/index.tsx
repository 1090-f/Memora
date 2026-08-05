'use client';

import React from 'react';
import { styled, Grid, alpha, Stack } from '@mui/material';
import { StyledTopicBox, StyledTopicTitle } from '../component/styledCommon';
import {
  useFadeInText,
  useCardFadeInAnimation,
} from '../hooks/useGsapAnimation';

interface MetricsProps {
  mobile?: boolean;
  title?: string;
  items?: {
    name: string;
    number: string;
  }[];
}

const StyledMetricsContainer = styled('div')(({ theme }) => ({
  width: '100%',
  border: `1px solid ${alpha(theme.palette.text.primary, 0.15)}`,
  borderRadius: '10px',
  padding: theme.spacing(3),
  boxShadow: `0px 5px 20px 0px ${alpha(theme.palette.text.primary, 0.06)}`,
}));

const StyledMetricsItemNumber = styled('div')(({ theme }) => ({
  fontSize: 48,
  fontWeight: 500,
  color: theme.palette.text.primary,
  userSelect: 'none',
  transition: 'color 0.2s ease',
  '&:hover': {
    color: theme.palette.primary.main,
  },
}));

const StyledMetricsItemTitle = styled('span')(({ theme }) => ({
  fontSize: 16,
  color: alpha(theme.palette.text.primary, 0.5),
  userSelect: 'none',
}));

// 单个卡片组件，带动画效果
const MetricsItem: React.FC<{
  item: {
    name: string;
    number: string;
  };
  index: number;
  size: any;
}> = React.memo(({ item, index, size }) => {
  const cardRef = useCardFadeInAnimation(0.2 + index * 0.1, 0.1);

  return (
    <Grid size={size} key={index}>
      <Stack
        ref={cardRef as React.Ref<HTMLDivElement>}
        gap={1}
        alignItems='center'
        sx={{ opacity: 0 }}
      >
        <StyledMetricsItemNumber className='metrics-item-number'>
          {item.number}
        </StyledMetricsItemNumber>

        <StyledMetricsItemTitle>{item.name}</StyledMetricsItemTitle>
      </Stack>
    </Grid>
  );
});

const Metrics: React.FC<MetricsProps> = React.memo(
  ({ title, items = [], mobile }) => {
    const size =
      typeof mobile === 'boolean'
        ? mobile
          ? 12
          : { xs: 12, md: items.length > 3 ? 4 : 12 / items.length }
        : { xs: 12, md: items.length > 3 ? 4 : 12 / items.length };

    // 添加标题淡入动画
    const titleRef = useFadeInText(0.2, 0.1);
    return (
      <StyledTopicBox>
        <StyledTopicTitle ref={titleRef}>{title}</StyledTopicTitle>
        <StyledMetricsContainer>
          <Grid container spacing={3} sx={{ width: '100%' }}>
            {items.map((item, index) => (
              <MetricsItem key={index} item={item} index={index} size={size} />
            ))}
          </Grid>
        </StyledMetricsContainer>
      </StyledTopicBox>
    );
  },
);

export default Metrics;
