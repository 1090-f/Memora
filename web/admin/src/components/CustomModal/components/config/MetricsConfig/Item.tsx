import { Box, IconButton, Stack, TextField } from '@mui/material';
import { IconShanchu2, IconDrag } from '@panda-wiki/icons';
import {
  CSSProperties,
  Dispatch,
  forwardRef,
  HTMLAttributes,
  SetStateAction,
} from 'react';

export type ItemType = {
  name: string;
  number: string;
  id: string;
};

export type ItemTypeProps = Omit<HTMLAttributes<HTMLDivElement>, 'onChange'> & {
  item: ItemType;
  withOpacity?: boolean;
  isDragging?: boolean;
  dragHandleProps?: React.HTMLAttributes<HTMLDivElement>;
  handleRemove?: (id: string) => void;
  handleUpdateItem?: (item: ItemType) => void;
  setIsEdit: Dispatch<SetStateAction<boolean>>;
};

const ItemType = forwardRef<HTMLDivElement, ItemTypeProps>(
  (
    {
      item,
      withOpacity,
      isDragging,
      style,
      dragHandleProps,
      handleRemove,
      handleUpdateItem,
      setIsEdit,
      ...props
    },
    ref,
  ) => {
    const inlineStyles: CSSProperties = {
      opacity: withOpacity ? '0.5' : '1',
      borderRadius: '10px',
      cursor: isDragging ? 'grabbing' : 'grab',
      backgroundColor: '#ffffff',
      width: '100%',
      ...style,
    };
    return (
      <Box ref={ref} style={inlineStyles} {...props}>
        <Stack
          direction={'row'}
          alignItems={'center'}
          justifyContent={'space-between'}
          gap={0.5}
          sx={{
            py: 1.5,
            px: 1,
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: '10px',
          }}
        >
          <Stack
            direction={'column'}
            gap={'20px'}
            sx={{
              flex: 1,
              p: 1.5,
            }}
          >
            <TextField
              label='指标数值'
              slotProps={{
                inputLabel: {
                  shrink: true,
                },
              }}
              sx={{
                height: '36px',
                '& .MuiOutlinedInput-root': {
                  height: '36px',
                  padding: '0 12px',
                  '& .MuiOutlinedInput-input': {
                    padding: '8px 0',
                  },
                },
              }}
              fullWidth
              placeholder='请输入指标数值'
              variant='outlined'
              value={item.number}
              onChange={e => {
                const updatedItem = { ...item, number: e.target.value };
                handleUpdateItem?.(updatedItem);
                setIsEdit(true);
              }}
            />
            <TextField
              label='指标名称'
              slotProps={{
                inputLabel: {
                  shrink: true,
                },
              }}
              sx={{
                height: '36px',
                '& .MuiOutlinedInput-root': {
                  height: '36px',
                  padding: '0 12px',
                  '& .MuiOutlinedInput-input': {
                    padding: '8px 0',
                  },
                },
              }}
              fullWidth
              placeholder='请输入指标名称'
              variant='outlined'
              value={item.name}
              onChange={e => {
                const updatedItem = { ...item, name: e.target.value };
                handleUpdateItem?.(updatedItem);
                setIsEdit(true);
              }}
            />
          </Stack>

          <Stack
            direction={'column'}
            sx={{ justifyContent: 'space-between', alignSelf: 'stretch' }}
          >
            <IconButton
              size='small'
              onClick={e => {
                e.stopPropagation();
                handleRemove?.(item.id);
              }}
              sx={{
                color: 'text.tertiary',
                ':hover': { color: 'error.main' },
                width: '28px',
                height: '28px',
              }}
            >
              <IconShanchu2 sx={{ fontSize: '12px' }} />
            </IconButton>
            <IconButton
              size='small'
              sx={{
                cursor: 'grab',
                color: 'text.secondary',
                '&:hover': { color: 'primary.main' },
              }}
              {...(dragHandleProps as any)}
            >
              <IconDrag sx={{ fontSize: '18px' }} />
            </IconButton>
          </Stack>
        </Stack>
      </Box>
    );
  },
);

export default ItemType;
