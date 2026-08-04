import {
  Box,
  IconButton,
  Stack,
  TextField,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
} from '@mui/material';
import { IconShanchu2, IconDrag } from '@panda-wiki/icons';
import {
  CSSProperties,
  Dispatch,
  forwardRef,
  HTMLAttributes,
  SetStateAction,
} from 'react';

type Item = {
  id: string;
  text: string;
  type: 'contained' | 'outlined' | 'text';
  href: string;
};

export type ItemProps = Omit<HTMLAttributes<HTMLDivElement>, 'onChange'> & {
  item: Item;
  withOpacity?: boolean;
  isDragging?: boolean;
  dragHandleProps?: React.HTMLAttributes<HTMLDivElement>;
  handleRemove?: (id: string) => void;
  handleUpdateItem?: (item: Item) => void;
  setIsEdit: Dispatch<SetStateAction<boolean>>;
};

const Item = forwardRef<HTMLDivElement, ItemProps>(
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
              label='文字'
              slotProps={{
                inputLabel: {
                  shrink: true,
                },
              }}
              size='small'
              fullWidth
              placeholder='请输入'
              variant='outlined'
              value={item.text}
              onChange={e => {
                const updatedItem = { ...item, text: e.target.value };
                handleUpdateItem?.(updatedItem);
                setIsEdit(true);
              }}
            />
            <TextField
              label='按钮链接'
              slotProps={{
                inputLabel: {
                  shrink: true,
                },
              }}
              size='small'
              fullWidth
              placeholder='请输入'
              variant='outlined'
              value={item.href}
              onChange={e => {
                const updatedItem = { ...item, href: e.target.value };
                handleUpdateItem?.(updatedItem);
                setIsEdit(true);
              }}
            />
            <FormControl>
              <InputLabel
                sx={{
                  '&.Mui-focused': {
                    color: 'black',
                  },
                }}
              >
                按钮样式
              </InputLabel>
              <Select
                value={item.type}
                label='按钮样式'
                onChange={e => {
                  const updatedItem = { ...item, type: e.target.value };
                  handleUpdateItem?.(updatedItem);
                  setIsEdit(true);
                }}
              >
                <MenuItem value={'contained'}>实心</MenuItem>
                <MenuItem value={'outlined'}>描边</MenuItem>
                <MenuItem value={'text'}>文字</MenuItem>
              </Select>
            </FormControl>
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

export default Item;
