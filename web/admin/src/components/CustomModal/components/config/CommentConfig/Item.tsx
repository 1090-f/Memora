import { Box, IconButton, Stack, TextField } from '@mui/material';
import { IconShanchu2, IconDrag } from '@panda-wiki/icons';
import {
  CSSProperties,
  Dispatch,
  forwardRef,
  HTMLAttributes,
  SetStateAction,
} from 'react';
import UploadFile from '@/components/UploadFile';

export type ItemType = {
  user_name: string;
  avatar: string;
  profession: string;
  comment: string;
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
              label='评论'
              slotProps={{
                inputLabel: {
                  shrink: true,
                },
              }}
              sx={{
                '& .MuiOutlinedInput-root': {
                  padding: '0 12px',
                  '& .MuiOutlinedInput-input': {
                    padding: '8px 0',
                  },
                },
              }}
              fullWidth
              multiline
              minRows={2}
              placeholder='请输入评论'
              variant='outlined'
              value={item.comment}
              onChange={e => {
                const updatedItem = { ...item, comment: e.target.value };
                handleUpdateItem?.(updatedItem);
                setIsEdit(true);
              }}
            />
            <TextField
              label='用户名'
              slotProps={{
                inputLabel: {
                  shrink: true,
                },
              }}
              sx={{
                '& .MuiOutlinedInput-root': {
                  padding: '0 12px',
                  '& .MuiOutlinedInput-input': {
                    padding: '8px 0',
                  },
                },
              }}
              fullWidth
              placeholder='请输入用户名'
              variant='outlined'
              value={item.user_name}
              onChange={e => {
                const updatedItem = { ...item, user_name: e.target.value };
                handleUpdateItem?.(updatedItem);
                setIsEdit(true);
              }}
            />
            <TextField
              label='职业'
              slotProps={{
                inputLabel: {
                  shrink: true,
                },
              }}
              sx={{
                '& .MuiOutlinedInput-root': {
                  padding: '0 12px',
                  '& .MuiOutlinedInput-input': {
                    padding: '8px 0',
                  },
                },
              }}
              fullWidth
              placeholder='请输入职业'
              variant='outlined'
              value={item.profession}
              onChange={e => {
                const updatedItem = { ...item, profession: e.target.value };
                handleUpdateItem?.(updatedItem);
                setIsEdit(true);
              }}
            />
            <UploadFile
              name='url'
              id={`${item.id}_icon`}
              type='url'
              disabled={false}
              accept='image/*'
              width={80}
              height={80}
              value={item.avatar}
              label='上传头像'
              onChange={(url: string) => {
                const updatedItem = { ...item, avatar: url };
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
