'use client';

import { styled } from '@mui/material';

const StyledContainer = styled('div')(({ theme }) => ({
  margin: '0 auto',
  width: '100%',
  maxWidth: 1248,
}));

export const StyledTopicContainer = styled('div')(({ theme }) => ({
  display: 'flex',
  justifyContent: 'center',
  gap: theme.spacing(8),
  alignItems: 'center',
  padding: theme.spacing(0, 2),
}));

export const StyledTopicInner = styled('div')(({ theme }) => ({
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  gap: theme.spacing(8),
  flex: 1,
  maxWidth: 1740,
  borderRadius: '20px',
  width: '100%',
}));

export const StyledTopicBox = styled(StyledContainer)(({ theme }) => ({
  display: 'flex',
  flexDirection: 'column',
  gap: theme.spacing(8),
  alignItems: 'center',
  padding: '90px 24px',
  [theme.breakpoints.down('md')]: {
    paddingTop: 60,
    paddingBottom: 60,
  },
}));

export const StyledEllipsis = styled('span')(({ theme }) => ({
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
  minWidth: 0,
}));

export const StyledTopicTitle = styled('h2')(({ theme }) => ({
  fontSize: 36,
  [theme.breakpoints.down('lg')]: {
    fontSize: 32,
  },
  [theme.breakpoints.down('md')]: {
    fontSize: 28,
  },
  fontWeight: 700,
  color: theme.palette.text.primary,
  overflow: 'hidden',
  opacity: 0,
}));

export const StyledTopicDes = styled('p')(({ theme }) => ({
  width: 998,
  fontSize: 18,
  fontWeight: 300,
  color: theme.palette.text.secondary,
  textAlign: 'center',
  [theme.breakpoints.down('md')]: {
    width: '100%',
  },
}));

export const StyledCard = styled('div')(({ theme }) => ({
  padding: `${theme.spacing(2)} `,
  backgroundColor: theme.palette.background.default,
  borderRadius: `calc(${theme.shape.borderRadius} * 2)`,
}));
