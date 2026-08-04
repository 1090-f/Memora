'use client';

import React from 'react';
import { styled, Stack, alpha } from '@mui/material';
import { StyledTopicBox, StyledTopicTitle } from '../component/styledCommon';
import { IconWenhao } from '@panda-wiki/icons';
import {
  useFadeInText,
  useCardFadeInAnimation,
} from '../hooks/useGsapAnimation';

interface QuestionProps {
  mobile?: boolean;
  title?: string;
  onSearch: (question: string) => void;
  items?: {
    question: string;
  }[];
}

const StyledItem = styled('div')(({ theme }) => ({
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
  cursor: 'pointer',
  opacity: 0,
}));

const StyledItemTitle = styled('span')(({ theme }) => ({
  fontSize: 20,
  fontWeight: 400,
}));

// 单个卡片组件，带动画效果
const Item: React.FC<{
  item: {
    question: string;
  };
  onSearch: (question: string) => void;
  index: number;
}> = React.memo(({ item, index, onSearch }) => {
  const cardRef = useCardFadeInAnimation(0.2 + index * 0.1, 0.1);

  return (
    <StyledItem
      ref={cardRef as React.Ref<HTMLDivElement>}
      onClick={() => onSearch(item.question)}
    >
      <IconWenhao sx={{ color: 'primary.main', fontSize: 20 }} />
      <StyledItemTitle>{item.question}</StyledItemTitle>
    </StyledItem>
  );
});

const Question: React.FC<QuestionProps> = React.memo(
  ({ title, items = [], onSearch }) => {
    // 添加标题淡入动画
    const titleRef = useFadeInText(0.2, 0.1);

    return (
      <StyledTopicBox>
        <StyledTopicTitle ref={titleRef}>{title}</StyledTopicTitle>
        <Stack gap={3} sx={{ width: '100%' }}>
          {items.map((item, index) => (
            <Item key={index} item={item} index={index} onSearch={onSearch} />
          ))}
        </Stack>
      </StyledTopicBox>
    );
  },
);

export default Question;
