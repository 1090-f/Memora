import { styled, Stack } from '@mui/material';
import { IconTianjia } from '@panda-wiki/icons';

export const StyledCommonWrapper = styled('div')(({ theme }) => ({
  display: 'flex',
  flexDirection: 'column',
  gap: theme.spacing(3),
}));

export const StyledCommonItemTitle = styled('div')(({ theme }) => ({
  fontSize: 14,
  lineHeight: '22px',
  flexShrink: 0,
  display: 'flex',
  alignItems: 'center',
  fontWeight: 600,
  '&::before': {
    content: '""',
    display: 'inline-block',
    width: 4,
    height: 12,
    backgroundColor: theme.palette.primary.main,
    borderRadius: '2px',
    marginRight: theme.spacing(1),
  },
}));

const StyledCommonItemTitleDesc = styled('div')(({ theme }) => ({
  fontSize: 12,
  fontWeight: 400,
  color: theme.palette.text.tertiary,
}));

export const StyledCommonItemTitleAdd = styled('div')(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  marginLeft: 'auto',
  cursor: 'pointer',
  gap: theme.spacing(0.5),
}));

export const StyledCommonItemTitleAddText = styled('div')(({ theme }) => ({
  fontSize: 14,
  lineHeight: '22px',
  marginLeft: 0.5,
  fontWeight: 400,
  color: theme.palette.text.secondary,
}));

export const CommonItem = ({
  children,
  title,
  onAdd,
  desc,
}: {
  children?: React.ReactNode;
  title?: string;
  desc?: string;
  onAdd?: () => void;
}) => {
  return (
    <Stack direction={'column'} gap={2}>
      <StyledCommonItemTitle>
        <Stack direction={'row'} alignItems={'center'} gap={1}>
          {title}
          {desc && (
            <StyledCommonItemTitleDesc>{desc}</StyledCommonItemTitleDesc>
          )}
        </Stack>

        {onAdd && (
          <StyledCommonItemTitleAdd onClick={onAdd}>
            <IconTianjia
              sx={{ fontSize: '10px !important', color: 'primary.main' }}
            />
            <StyledCommonItemTitleAddText>添加</StyledCommonItemTitleAddText>
          </StyledCommonItemTitleAdd>
        )}
      </StyledCommonItemTitle>

      {children}
    </Stack>
  );
};
