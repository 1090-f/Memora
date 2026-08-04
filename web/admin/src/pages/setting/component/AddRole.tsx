import { Box, Tooltip, Stack, Select, MenuItem, Radio } from '@mui/material';
import { getApiV1UserList } from '@/request/User';
import { postApiV1KnowledgeBaseUserInvite } from '@/request/KnowledgeBase';
import {
  ConstsUserKBPermission,
  V1KBUserInviteReq,
  V1UserListItemResp,
} from '@/request/types';
import { FormItem } from '@/components/Form';
import NoData from '@/assets/images/nodata.png';
import Card from '@/components/Card';
import { message, Modal, Table } from '@ctzhian/ui';
import dayjs from 'dayjs';
import { ColumnType } from '@ctzhian/ui/dist/Table';
import { useEffect, useMemo, useState } from 'react';
import { useAppSelector } from '@/store';
import { VersionCanUse } from '@/components/VersionMask';
import { PROFESSION_VERSION_PERMISSION } from '@/constant/version';

interface AddRoleProps {
  open: boolean;
  onCancel: () => void;
  onOk: () => void;
  selectedIds: string[];
}

const AddRole = ({ open, onCancel, onOk, selectedIds }: AddRoleProps) => {
  const { kb_id } = useAppSelector(state => state.config);
  const { license } = useAppSelector(state => state.config);
  const [list, setList] = useState<V1UserListItemResp[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string>('');
  const [perm, setPerm] = useState<V1KBUserInviteReq['perm']>(
    ConstsUserKBPermission.UserKBPermissionFullControl,
  );

  const columns: ColumnType<V1UserListItemResp>[] = [
    {
      title: '',
      dataIndex: 'id',
      width: 80,
      render: (text: string) => (
        <Tooltip
          arrow
          placement='top'
          title={selectedIds.includes(text) ? '已添加' : ''}
        >
          <span>
            <Radio
              disableRipple
              size='small'
              disabled={selectedIds.includes(text)}
              checked={selectedRowKeys === text}
              onChange={() => {
                setSelectedRowKeys(text);
              }}
              sx={{
                '.MuiTouchRipple-root': {
                  display: 'none',
                },
              }}
            />
          </span>
        </Tooltip>
      ),
    },
    {
      title: '用户名',
      dataIndex: 'account',
      render: (text: string) => (
        <Stack direction={'row'} alignItems={'center'} gap={2}>
          {text}
        </Stack>
      ),
    },
    {
      title: '上次使用时间',
      dataIndex: 'last_access',
      render: (text: string) => (
        <Box>{text ? dayjs(text).format('YYYY-MM-DD HH:mm:ss') : '-'}</Box>
      ),
    },
  ];
  const getData = () => {
    setLoading(true);
    getApiV1UserList()
      .then(res => {
        setList(res.users || []);
      })
      .finally(() => {
        setLoading(false);
      });
  };

  const onSubmit = () => {
    if (!selectedRowKeys) {
      message.error('请选择用户');
      return;
    }
    postApiV1KnowledgeBaseUserInvite({
      kb_id,
      user_id: selectedRowKeys,
      perm,
    }).then(() => {
      onOk();
      message.success('添加成功');
    });
  };

  useEffect(() => {
    if (open) {
      getData();
    } else {
      setSelectedRowKeys('');
      setPerm(
        ConstsUserKBPermission.UserKBPermissionFullControl as V1KBUserInviteReq['perm'],
      );
    }
  }, [open]);

  const isPro = useMemo(() => {
    return PROFESSION_VERSION_PERMISSION.includes(license.edition!);
  }, [license.edition]);

  return (
    <Modal
      title='添加 Wiki 站管理员'
      open={open}
      onCancel={onCancel}
      onOk={onSubmit}
      width={800}
    >
      <Card
        sx={{
          py: 2,
          border: '1px solid',
          borderColor: 'divider',
        }}
      >
        <Table
          columns={columns}
          dataSource={list}
          rowKey='id'
          size='small'
          updateScrollTop={false}
          sx={{
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
            '.MuiTableHead-root .cx-selection-column .MuiCheckbox-root': {
              visibility: 'hidden',
            },
          }}
          pagination={false}
          // rowSelection={{
          //   hideSelectAll: true,
          //   selectedRowKeys: selectedRowKeys,
          //   getCheckboxProps: (record: V1UserListItemResp) => {
          //     return {
          //       disabled:
          //         selectedRowKeys.length > 0
          //           ? !selectedRowKeys.includes(record.id!)
          //           : false,
          //     };
          //   },
          //   // @ts-expect-error 类型错误
          //   onChange: (selectedRowKeys: string[]) => {
          //     setSelectedRowKeys(selectedRowKeys);
          //   },
          // }}
          renderEmpty={
            loading ? (
              <Box></Box>
            ) : (
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
            )
          }
        />
      </Card>
      <FormItem
        label={
          <Stack
            sx={{ display: 'inline-flex' }}
            direction={'row'}
            alignItems={'center'}
            gap={0.5}
          >
            权限
          </Stack>
        }
        sx={{ mt: 2 }}
      >
        <Select
          fullWidth
          sx={{ height: 52 }}
          value={perm}
          MenuProps={{
            sx: {
              '.Mui-disabled': {
                opacity: '1 !important',
                color: 'text.disabled',
              },
            },
          }}
          onChange={e => setPerm(e.target.value as V1KBUserInviteReq['perm'])}
        >
          <MenuItem value={ConstsUserKBPermission.UserKBPermissionFullControl}>
            完全控制
          </MenuItem>

          <MenuItem
            value={ConstsUserKBPermission.UserKBPermissionDocManage}
            disabled={!isPro}
          >
            文档管理{' '}
            <VersionCanUse permission={PROFESSION_VERSION_PERMISSION} />
          </MenuItem>
          <MenuItem
            value={ConstsUserKBPermission.UserKBPermissionDataOperate}
            disabled={!isPro}
          >
            数据运营{' '}
            <VersionCanUse permission={PROFESSION_VERSION_PERMISSION} />
          </MenuItem>
        </Select>
      </FormItem>
    </Modal>
  );
};

export default AddRole;
