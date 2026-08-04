'use client';

import React from 'react';
import { styled, Grid, alpha } from '@mui/material';
import { StyledTopicBox, StyledTopicTitle } from '../component/styledCommon';
import { IconLianjiezu } from '@panda-wiki/icons';
import {
  useFadeInText,
  useCardFadeInAnimation,
} from '../hooks/useGsapAnimation';

interface FaqProps {
  mobile?: boolean;
  title?: string;
  items?: {
    question: string;
    url: string;
  }[];
}

const StyledFaqItem = styled('a')(({ theme }) => ({
  position: 'relative',
  overflow: 'hidden',
  display: 'flex',
  alignItems: 'center',
  gap: theme.spacing(2),
  color: theme.palette.text.primary,
  borderRadius: '10px',
  border: `1px solid ${alpha(theme.palette.text.primary, 0.1)}`,
  boxShadow: `0px 5px 20px 0px ${alpha(theme.palette.text.primary, 0.06)}`,
  padding: theme.spacing(3, 4),
  transition: 'all 0.2s ease',
  '&:hover': {
    transform: 'translateY(-5px)',
    color: theme.palette.primary.main,
    border: `1px solid ${alpha(theme.palette.primary.main, 0.5)}`,
    boxShadow: `0px 10px 20px 0px ${alpha(theme.palette.text.primary, 0.1)}`,
  },
  whiteSpace: 'nowrap',
  textOverflow: 'ellipsis',
  opacity: 0,
}));

const StyledFaqItemTitle = styled('span')(({ theme }) => ({
  fontSize: 16,
  fontWeight: 400,
}));

// 单个卡片组件，带动画效果
const FaqItem: React.FC<{
  item: any;
  index: number;
  size: any;
}> = React.memo(({ item, index, size }) => {
  const cardRef = useCardFadeInAnimation(0.2 + index * 0.1, 0.1);

  return (
    <Grid size={size} key={index}>
      <StyledFaqItem
        ref={cardRef as React.Ref<HTMLAnchorElement>}
        href={item.url}
        target='_blank'
      >
        <IconLianjiezu sx={{ color: 'primary.main', fontSize: 18 }} />
        <StyledFaqItemTitle>{item.question}</StyledFaqItemTitle>
      </StyledFaqItem>
    </Grid>
  );
});

const Faq: React.FC<FaqProps> = React.memo(({ title, items = [], mobile }) => {
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
          <FaqItem key={index} item={item} index={index} size={size} />
        ))}
      </Grid>
    </StyledTopicBox>
  );
});

export default Faq;
