import { ITreeItem } from '@/api';
import Cascader from '@/components/Cascader';
import { addOpacityToColor } from '@/utils';
import { Box, IconButton, Stack, useTheme } from '@mui/material';
import { IconXiala, IconGengduo } from '@panda-wiki/icons';

export type TreeMenuItem = {
  key: string;
  label: string;
  onClick?: () => void;
  disabled?: boolean;
  color?: 'error' | 'default';
  children?: {
    key: string;
    label: string;
    disabled?: boolean;
    onClick?: () => void;
    color?: 'error' | 'default';
  }[];
};

export type TreeMenuOptions = {
  item: ITreeItem;
  createItem: (type: 1 | 2, contentType?: string) => void;
  renameItem: () => void;
  isEditing: boolean;
  removeItem: (id: string) => void;
};

const TreeMenu = ({
  menu,
  context,
}: {
  menu: TreeMenuItem[];
  context?: React.ReactElement<{ onClick?: any; 'aria-describedby'?: any }>;
}) => {
  const theme = useTheme();

  return (
    <Cascader
      anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
      transformOrigin={{ vertical: 'top', horizontal: 'right' }}
      childrenProps={{
        anchorOrigin: { vertical: 'top', horizontal: 'left' },
        transformOrigin: { vertical: 'top', horizontal: 'right' },
      }}
      list={menu?.map(value => ({
        key: value.key,
        children: value.children?.map(it => ({
          key: it.key,
          onClick: it?.disabled ? undefined : it.onClick,
          label: (
            <Box key={it.key}>
              <Stack
                direction={'row'}
                alignItems={'center'}
                gap={1}
                sx={{
                  fontSize: 14,
                  px: 2,
                  lineHeight: '40px',
                  height: 40,
                  width: 180,
                  borderRadius: '5px',
                  cursor: 'pointer',
                  ':hover': {
                    bgcolor: addOpacityToColor(theme.palette.primary.main, 0.1),
                  },
                }}
              >
                {it.label}
              </Stack>
            </Box>
          ),
        })),
        label: (
          <Box key={value.key}>
            <Stack
              direction={'row'}
              alignItems={'center'}
              justifyContent={'space-between'}
              gap={1}
              sx={{
                fontSize: 14,
                pl: 2,
                pr: 1,
                lineHeight: '40px',
                height: 40,
                width: 180,
                borderRadius: '5px',
                color: value?.disabled
                  ? 'text.disabled'
                  : value.color === 'error'
                    ? 'error.main'
                    : 'text.primary',
                cursor: value?.disabled ? 'not-allowed' : 'pointer',
                ':hover': {
                  bgcolor: value?.disabled
                    ? 'transparent'
                    : value.color === 'error'
                      ? addOpacityToColor(theme.palette.error.main, 0.1)
                      : addOpacityToColor(theme.palette.primary.main, 0.1),
                },
              }}
              onClick={value?.disabled ? undefined : value.onClick}
            >
              {value.label}
              {value.children && (
                <IconXiala sx={{ fontSize: 20, transform: 'rotate(-90deg)' }} />
              )}
            </Stack>
            {value.key === 'next-line' && (
              <Box
                sx={{
                  width: 145,
                  borderBottom: '1px dashed',
                  borderColor: 'divider',
                  my: 0.5,
                  mx: 'auto',
                }}
              />
            )}
          </Box>
        ),
      }))}
      context={
        context || (
          <IconButton size='small'>
            <IconGengduo sx={{ fontSize: '14px' }} />
          </IconButton>
        )
      }
    />
  );
};

export default TreeMenu;
