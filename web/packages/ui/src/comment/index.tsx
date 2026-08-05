'use client';

import React from 'react';
import { styled, Grid, alpha, Stack, Rating } from '@mui/material';
import { StyledTopicBox, StyledTopicTitle } from '../component/styledCommon';
import {
  useFadeInText,
  useCardFadeInAnimation,
} from '../hooks/useGsapAnimation';

interface Props {
  mobile?: boolean;
  title?: string;
  items: {
    user_name?: string;
    profession?: string;
    avatar?: string;
    comment?: string;
  }[];
}
const StyledItem = styled(Stack)(({ theme }) => ({
  border: `1px solid ${alpha(theme.palette.text.primary, 0.15)}`,
  borderRadius: '10px',
  padding: theme.spacing(3),
  boxShadow: `0px 5px 20px 0px ${alpha(theme.palette.text.primary, 0.06)}`,
  height: '100%',
  justifyContent: 'space-between',
  transition: 'all 0.2s ease',
  '&:hover': {
    color: theme.palette.primary.main,
    borderColor: theme.palette.primary.main,
    boxShadow: `0px 10px 20px 0px ${alpha(theme.palette.text.primary, 0.1)}`,
  },
  opacity: 0,
}));

const StyledItemSummary = styled('div')(({ theme }) => ({
  fontSize: 16,
  fontWeight: 400,
  color: alpha(theme.palette.text.primary, 0.85),
}));

const StyledItemUserAvatar = styled('img')(({ theme }) => ({
  width: 40,
  height: 40,
  borderRadius: '50%',
}));

const StyledItemUser = styled('div')(({ theme }) => ({
  fontSize: 14,
  fontWeight: 400,
  color: theme.palette.text.primary,
}));

const StyledItemProfession = styled('div')(({ theme }) => ({
  fontSize: 12,
  fontWeight: 400,
  color: alpha(theme.palette.text.primary, 0.5),
}));

// 单个卡片组件，带动画效果
const Item: React.FC<{
  item: {
    comment?: string;
    avatar?: string;
    user_name?: string;
    profession?: string;
  };
  index: number;
}> = React.memo(({ item, index }) => {
  const cardRef = useCardFadeInAnimation(0.2 + index * 0.1, 0.1);
  return (
    <StyledItem ref={cardRef as React.Ref<HTMLDivElement>} gap={3}>
      <StyledItemSummary>{item.comment}</StyledItemSummary>
      <Stack direction='row' gap={1}>
        {item.avatar && (
          <StyledItemUserAvatar src={item.avatar} alt={item.user_name} />
        )}
        <Stack gap={0.5} justifyContent='center'>
          <StyledItemUser>{item.user_name}</StyledItemUser>
          <StyledItemProfession>{item.profession}</StyledItemProfession>
        </Stack>
      </Stack>
    </StyledItem>
  );
});

const Comment: React.FC<Props> = React.memo(({ title, items, mobile }) => {
  // 添加标题淡入动画
  const titleRef = useFadeInText(0.2, 0.1);
  const size =
    typeof mobile === 'boolean'
      ? mobile
        ? 12
        : { xs: 12, md: 4 }
      : { xs: 12, md: 4 };

  return (
    <StyledTopicBox>
      <StyledTopicTitle ref={titleRef}>{title}</StyledTopicTitle>
      <Grid container spacing={3} sx={{ width: '100%' }}>
        {items.map((item, index) => (
          <Grid size={size} key={index}>
            <Item item={item} index={index} />
          </Grid>
        ))}
      </Grid>
    </StyledTopicBox>
  );
});

export default Comment;
