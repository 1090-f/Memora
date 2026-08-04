'use client';

import React, { useMemo } from 'react';
import { styled, alpha, Stack, Box } from '@mui/material';
import { StyledTopicBox, StyledTopicTitle } from '../component/styledCommon';
import { useFadeInText, useCardAnimation } from '../hooks/useGsapAnimation';

interface ImgTextProps {
  mobile?: boolean;
  title?: string;
  direction?: 'row' | 'row-reverse';
  item: {
    name: string;
    url: string;
    desc: string;
  };
}
const StyledImgTextItem = styled(Stack)(({ theme }) => ({}));

export const StyledImgTextItemImg = styled('img')(({ theme }) => ({
  maxWidth: '100%',
  maxHeight: '100%',
  width: '100%',
  height: '100%',
  objectFit: 'cover',
  flex: '0 0 auto',
  borderRadius: '10px',
}));

const StyledImgTextItemTitle = styled('h3')(({ theme }) => ({
  fontSize: 24,
  fontWeight: 700,
  color: theme.palette.text.primary,
}));

const StyledImgTextItemSummary = styled('div')(({ theme }) => ({
  fontSize: 16,
  fontWeight: 400,
  color: alpha(theme.palette.text.primary, 0.5),
}));

const ImgText: React.FC<ImgTextProps> = React.memo(
  ({ title, mobile, item, direction = 'row' }) => {
    const size =
      typeof mobile === 'boolean'
        ? mobile
          ? 12
          : { xs: 12, md: 6 }
        : { xs: 12, md: 6 };

    const titleRef = useFadeInText(0.2, 0.1);

    const cardLeftAnimation = useMemo(
      () => ({
        initial: { opacity: 0, x: -250 },
        to: { opacity: 1, x: 0, duration: 0.6, ease: 'power2.out' },
      }),
      [],
    );

    const cardRightAnimation = useMemo(
      () => ({
        initial: { opacity: 0, x: 250 },
        to: { opacity: 1, x: 0, duration: 0.6, ease: 'power2.out' },
      }),
      [],
    );

    const cardLeftRef = useCardAnimation(cardLeftAnimation);
    const cardRightRef = useCardAnimation(cardRightAnimation);

    return (
      <StyledTopicBox>
        <StyledTopicTitle ref={titleRef}>{title}</StyledTopicTitle>
        <StyledImgTextItem
          gap={mobile ? 4 : { xs: 4, sm: 6, md: 16 }}
          direction={
            mobile
              ? 'column-reverse'
              : {
                  xs: 'column-reverse',
                  md: direction,
                }
          }
          alignItems='center'
          justifyContent='center'
          sx={{ width: '100%' }}
        >
          <Box
            sx={{ width: '100%' }}
            ref={cardLeftRef as React.Ref<HTMLDivElement>}
          >
            <StyledImgTextItemImg src={item.url} alt={item.name} />
          </Box>
          <Stack
            gap={1}
            sx={{ width: '100%' }}
            ref={cardRightRef as React.Ref<HTMLDivElement>}
            alignItems={
              mobile
                ? 'flex-start'
                : direction === 'row'
                  ? 'flex-start'
                  : 'flex-end'
            }
          >
            <StyledImgTextItemTitle
              sx={{
                textAlign: mobile
                  ? 'left'
                  : direction === 'row'
                    ? 'left'
                    : 'right',
              }}
            >
              {item.name}
            </StyledImgTextItemTitle>
            <StyledImgTextItemSummary
              sx={{
                textAlign: mobile
                  ? 'left'
                  : direction === 'row'
                    ? 'left'
                    : 'right',
              }}
            >
              {item.desc}
            </StyledImgTextItemSummary>
          </Stack>
        </StyledImgTextItem>
      </StyledTopicBox>
    );
  },
);

export default ImgText;
