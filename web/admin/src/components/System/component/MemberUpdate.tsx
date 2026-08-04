import Card from '@/components/Card';
import { putApiV1UserResetPassword } from '@/request/User';
import { V1UserListItemResp } from '@/request/types';
import { copyText, generatePassword } from '@/utils';
import { Modal } from '@ctzhian/ui';
import { CheckCircle } from '@mui/icons-material';
import { Box, IconButton, Stack, TextField } from '@mui/material';
import { IconShuaxin } from '@panda-wiki/icons';
import { useEffect, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';

type UpdateMemberProps = {
  user: V1UserListItemResp;
  refresh: () => void;
  type: 'reset' | 'update';
};

const MemberUpdate = ({ user, refresh, type }: UpdateMemberProps) => {
  const [updateOpen, setUpdateOpen] = useState(false);
  const [resetOpen, setResetOpen] = useState(false);
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const {
    control,
    handleSubmit,
    formState: { errors },
    reset,
    setValue,
  } = useForm({
    defaultValues: {
      password: '',
    },
  });

  const close = () => {
    setResetOpen(false);
    setUpdateOpen(false);
    setPassword('');
    reset();
    setLoading(false);
    refresh();
  };

  const copyUserInfo = () => {
    copyText(`用户名: ${user.account}\n密码: ${password}`, close);
  };

  const onSumbit = (data: { password: string }) => {
    setLoading(true);
    putApiV1UserResetPassword({ id: user.id!, new_password: data.password })
      .then(() => {
        setPassword(data.password);
        setUpdateOpen(false);
        setResetOpen(true);
      })
      .finally(() => {
        setLoading(false);
      });
  };

  useEffect(() => {
    if (type === 'reset') {
      const password = generatePassword();
      setPassword(password);
    }
    setUpdateOpen(true);
  }, [user, type]);

  return (
    <>
      <Modal
        title={
          <Stack direction='row' alignItems='center' gap={1}>
            <CheckCircle sx={{ color: 'success.main' }} />
            密码修改成功
          </Stack>
        }
        open={resetOpen}
        onCancel={close}
        okText={'复制用户信息'}
        cancelText={'关闭'}
        closable={false}
        okButtonProps={{ sx: { minWidth: '120px' } }}
        onOk={copyUserInfo}
      >
        <Card sx={{ p: 2, fontSize: 14, bgcolor: 'background.paper3' }}>
          <Stack direction={'row'}>
            <Box sx={{ width: 80 }}>用户名</Box>
            <Box sx={{ fontWeight: 700 }}>{user.account}</Box>
          </Stack>
          <Stack direction={'row'} sx={{ mt: 1 }}>
            <Box sx={{ width: 80 }}>{'新密码'}</Box>
            <Box sx={{ fontWeight: 700 }}>{password}</Box>
          </Stack>
        </Card>
      </Modal>
      <Modal
        title={type === 'reset' ? '重置密码？' : '修改密码'}
        open={updateOpen}
        onOk={handleSubmit(onSumbit)}
        onCancel={close}
        okButtonProps={{
          loading,
        }}
      >
        <Box sx={{ fontSize: 14, lineHeight: '32px' }}>
          用户名{' '}
          <Box component={'span'} sx={{ color: 'red', mb: 1 }}>
            *
          </Box>
        </Box>
        <Box
          sx={{
            lineHeight: '36px',
            bgcolor: 'background.paper3',
            px: '14px',
            borderRadius: '10px',
            fontSize: 14,
            fontWeight: 700,
            cursor: 'not-allowed',
          }}
        >
          {user.account}
        </Box>
        <Box sx={{ fontSize: 14, lineHeight: '32px', mt: 2, mb: 1 }}>
          密码{' '}
          <Box component={'span'} sx={{ color: 'red' }}>
            *
          </Box>
        </Box>
        <Stack direction={'row'} alignItems={'center'} gap={2}>
          <Controller
            control={control}
            name='password'
            rules={{
              required: {
                value: true,
                message: '密码不能为空',
              },
              minLength: {
                value: 8,
                message: '密码长度至少 8 位',
              },
            }}
            render={({ field }) => (
              <TextField
                {...field}
                fullWidth
                autoFocus
                size='small'
                placeholder='输入密码'
                error={!!errors.password}
                helperText={errors.password?.message}
              />
            )}
          />
          <IconButton
            color='primary'
            size='small'
            onClick={() => setValue('password', generatePassword())}
            sx={{ flexShrink: 0 }}
          >
            <IconShuaxin sx={{ fontSize: 16 }} />
          </IconButton>
        </Stack>
      </Modal>
    </>
  );
};

export default MemberUpdate;
