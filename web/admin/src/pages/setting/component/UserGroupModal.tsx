import { useEffect, useMemo, useState, KeyboardEvent, useRef } from 'react';
import {
  postApiProV1AuthGroupCreate,
  patchApiProV1AuthGroupUpdate,
} from '@/request/pro/AuthGroup';
import {
  TextField,
  Box,
  Stack,
  Tooltip,
  IconButton,
  Button,
  ClickAwayListener,
} from '@mui/material';
import dayjs from 'dayjs';
import { Modal, Table, message } from '@ctzhian/ui';
import NoData from '@/assets/images/nodata.png';
import { useForm, Controller } from 'react-hook-form';
import { FormItem } from './Common';
import { useAppSelector } from '@/store';
import { ColumnType } from '@ctzhian/ui/dist/Table';
import SearchIcon from '@mui/icons-material/Search';
import {
  GithubComChaitinPandaWikiProApiAuthV1AuthGroupListItem,
  GithubComChaitinPandaWikiProApiAuthV1AuthItem,
} from '@/request/pro/types';

interface UserGroupModalProps {
  data?: GithubComChaitinPandaWikiProApiAuthV1AuthGroupListItem;
  open: boolean;
  onCancel: () => void;
  onOk: () => void;
  userList: GithubComChaitinPandaWikiProApiAuthV1AuthItem[];
  type: 'add' | 'edit';
}

const UserGroupModal = ({
  data,
  open,
  onCancel,
  userList,
  onOk,
  type,
}: UserGroupModalProps) => {
  const { kb_id } = useAppSelector(state => state.config);
  const [selectedRowKeys, setSelectedRowKeys] = useState<number[]>(
    data?.auth_ids || [],
  );
  const [usernameFilterOpen, setUsernameFilterOpen] = useState(false);
  const [usernameFilter, setUsernameFilter] = useState('');
  const [usernameInput, setUsernameInput] = useState('');
  const usernameTooltipContentRef = useRef<HTMLDivElement | null>(null);
  const hasUsernameFilter = !!usernameFilter || usernameInput.trim().length > 0;

  const handleApplyUsernameFilter = () => {
    setUsernameFilter(usernameInput.trim());
    setUsernameFilterOpen(false);
  };

  const handleResetUsernameFilter = () => {
    setUsernameInput('');
    setUsernameFilter('');
    setUsernameFilterOpen(false);
  };

  const handleUsernameClickAway = (event: MouseEvent | TouchEvent) => {
    if (usernameTooltipContentRef.current?.contains(event.target as Node)) {
      return;
    }
    setUsernameFilterOpen(false);
  };

  const handleUsernameInputEnter = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Enter') {
      event.preventDefault();
      handleApplyUsernameFilter();
    }
  };

  const filteredUserList = useMemo(() => {
    if (!usernameFilter) return userList;
    return userList.filter(user =>
      (user.username || '')
        .toLowerCase()
        .includes(usernameFilter.toLowerCase()),
    );
  }, [userList, usernameFilter]);
  const columns: ColumnType<GithubComChaitinPandaWikiProApiAuthV1AuthItem>[] = [
    {
      title: (
        <Stack direction={'row'} alignItems={'center'} gap={0.5}>
          <Box component={'span'}>用户名</Box>
          <ClickAwayListener onClickAway={handleUsernameClickAway}>
            <Tooltip
              open={usernameFilterOpen}
              disableFocusListener
              disableHoverListener
              disableTouchListener
              title={
                <Stack gap={1} ref={usernameTooltipContentRef}>
                  <TextField
                    size='small'
                    autoFocus
                    value={usernameInput}
                    onChange={e => setUsernameInput(e.target.value)}
                    onKeyDown={handleUsernameInputEnter}
                    placeholder='输入用户名筛选'
                    sx={{ minWidth: 200 }}
                  />
                  <Stack
                    direction={'row'}
                    justifyContent={'space-between'}
                    alignItems={'center'}
                    gap={1}
                  >
                    <Button
                      variant='text'
                      size='small'
                      color='primary'
                      onClick={handleResetUsernameFilter}
                      disabled={!hasUsernameFilter}
                    >
                      重置
                    </Button>
                    <Button
                      variant='contained'
                      size='small'
                      onClick={handleApplyUsernameFilter}
                    >
                      确定
                    </Button>
                  </Stack>
                </Stack>
              }
              slotProps={{
                tooltip: {
                  sx: {
                    bgcolor: 'background.paper',
                    color: 'text.primary',
                    boxShadow: 3,
                    p: 1.5,
                  },
                },
              }}
            >
              <IconButton
                size='small'
                color={usernameFilter ? 'primary' : 'default'}
                sx={{ ml: 0.5 }}
                onClick={event => {
                  event.stopPropagation();
                  setUsernameInput(usernameFilter);
                  setUsernameFilterOpen(prev => !prev);
                }}
              >
                <SearchIcon sx={{ fontSize: 16 }} />
              </IconButton>
            </Tooltip>
          </ClickAwayListener>
        </Stack>
      ),
      dataIndex: 'username',
      render: (text: string) => {
        return (
          <Stack direction={'row'} alignItems={'center'} gap={1}>
            {text}
          </Stack>
        );
      },
    },
    {
      title: '时间',
      dataIndex: 'created_at',
      render: (text: string, record) => {
        return (
          <Box sx={{ color: 'text.secondary' }}>
            {dayjs(text).fromNow()}加入，
            {dayjs(record.last_login_time).fromNow()}活跃
          </Box>
        );
      },
    },
  ];

  const {
    control,
    handleSubmit,
    reset,
    setValue,
    formState: { errors },
  } = useForm({
    defaultValues: {
      name: '',
    },
  });
  const onSubmit = handleSubmit(values => {
    if (type === 'edit' && data) {
      patchApiProV1AuthGroupUpdate({
        name: values.name,
        auth_ids: selectedRowKeys,
        kb_id,
        id: data.id!,
      }).then(res => {
        message.success('编辑成功');
        onOk();
      });
    } else if (type === 'add') {
      postApiProV1AuthGroupCreate({
        name: values.name,
        ids: selectedRowKeys,
        kb_id,
        parent_id:
          (data?.id as 'root' | number) === 'root' ? undefined : data?.id,
      }).then(res => {
        message.success('添加成功');
        onOk();
      });
    }
  });

  useEffect(() => {
    if (type === 'edit' && data) {
      setSelectedRowKeys(data?.auth_ids || []);
      setValue('name', data?.name || '');
    }
  }, [data, type]);

  useEffect(() => {
    if (!open) {
      reset();
      setSelectedRowKeys([]);
      setUsernameFilter('');
      setUsernameInput('');
      setUsernameFilterOpen(false);
    }
  }, [open]);

  return (
    <Modal
      title={type === 'add' ? '添加用户组' : '编辑用户组'}
      open={open}
      onCancel={onCancel}
      onOk={onSubmit}
      width={660}
    >
      <FormItem label='用户组名称' vertical required>
        <Controller
          control={control}
          name='name'
          rules={{ required: '请输入用户组名称' }}
          render={({ field }) => (
            <TextField
              {...field}
              fullWidth
              placeholder='请输入用户组名称'
              helperText={errors.name?.message}
              error={!!errors.name}
            />
          )}
        />
      </FormItem>
      <Table
        columns={columns}
        dataSource={filteredUserList}
        rowKey='id'
        size='small'
        updateScrollTop={false}
        sx={{
          mt: 2,
          '.MuiTableContainer-root': {
            maxHeight: 'calc(100vh - 370px)',
            minHeight: 200,
          },
          '& .MuiTableCell-root': {
            height: 40,
            '&:first-of-type': {
              pl: 2,
            },
          },
        }}
        pagination={false}
        rowSelection={{
          hideSelectAll: true,
          selectedRowKeys: selectedRowKeys,
          // @ts-expect-error 类型错误
          onChange: (selectedRowKeys: number[]) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
        renderEmpty={
          <Stack alignItems={'center'}>
            <img src={NoData} width={150} />
            <Box
              sx={{
                fontSize: 12,
                lineHeight: '20px',
                color: 'text.tertiary',
              }}
            >
              暂无数据
            </Box>
          </Stack>
        }
      />
    </Modal>
  );
};

export default UserGroupModal;
