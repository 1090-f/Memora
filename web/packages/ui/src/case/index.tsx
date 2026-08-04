'use client';

import React from 'react';
import { styled, Grid, alpha, Stack } from '@mui/material';
import { StyledTopicBox, StyledTopicTitle } from '../component/styledCommon';
import {
  useFadeInText,
  useCardScaleAnimation,
} from '../hooks/useGsapAnimation';

interface CaseProps {
  mobile?: boolean;
  title?: string;
  bgColor?: string;
  titleColor?: string;
  items?: {
    name: string;
    link: string;
  }[];
}
const StyledCaseItem = styled('a')(({ theme }) => ({
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
  cursor: 'pointer',
  opacity: 0,
  scale: 0,
}));

const StyledCaseItemTitle = styled('span')(({ theme }) => ({
  fontSize: 16,
  fontWeight: 400,
  color: theme.palette.text.primary,
}));

// 单个卡片组件，带动画效果
const CaseItem: React.FC<{
  item: any;
  index: number;
}> = React.memo(({ item, index }) => {
  const rand = Math.random();
  const cardRef = useCardScaleAnimation({
    duration: rand < 0.5 ? rand + 0.5 : rand,
  });
  return (
    <StyledCaseItem
      ref={cardRef as React.Ref<HTMLAnchorElement>}
      href={item.link}
      target='_blank'
    >
      <StyledCaseItemTitle>{item.name}</StyledCaseItemTitle>
    </StyledCaseItem>
  );
});

const Case: React.FC<CaseProps> = React.memo(
  ({ title = '案例', items = [], mobile }) => {
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
        <Stack
          gap={1}
          direction='row'
          alignItems='center'
          justifyContent='center'
          flexWrap='wrap'
        >
          {items.map((item, index) => (
            <CaseItem key={index} item={item} index={index} />
          ))}
        </Stack>
      </StyledTopicBox>
    );
  },
);

export default Case;
