import { postApiV1UserCreate } from '@/request/User';
import { postApiV1KnowledgeBaseUserInvite } from '@/request/KnowledgeBase';
import Card from '@/components/Card';
import { copyText, generatePassword } from '@/utils';
import { CheckCircle } from '@mui/icons-material';
import { Box, Button, MenuItem, Select, Stack, TextField } from '@mui/material';
import { FormItem } from '@/components/Form';
import { Modal, message } from '@ctzhian/ui';
import { useState, useMemo, useEffect } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { useAppSelector } from '@/store';
import { PROFESSION_VERSION_PERMISSION } from '@/constant/version';
import { VersionCanUse } from '@/components/VersionMask';
import { ConstsUserKBPermission, V1KBUserInviteReq } from '@/request/types';
import { ConstsLicenseEdition } from '@/request/pro/types';

type Role = 'admin' | 'user';

const PERM_MAP = {
  [ConstsUserKBPermission.UserKBPermissionFullControl]: '完全控制',
  [ConstsUserKBPermission.UserKBPermissionDocManage]: '文档管理',
  [ConstsUserKBPermission.UserKBPermissionDataOperate]: '数据运营',
};

const VERSION_MAP = {
  [ConstsLicenseEdition.LicenseEditionFree]: {
    message: '开源版只支持 1 个管理员',
    max: 1,
  },
  [ConstsLicenseEdition.LicenseEditionProfession]: {
    message: '专业版最多支持 20 个管理员',
    max: 20,
  },
  [ConstsLicenseEdition.LicenseEditionBusiness]: {
    message: '商业版最多支持 50 个管理员',
    max: 50,
  },
};

const MemberAdd = ({
  refresh,
  userLen,
}: {
  refresh: () => void;
  userLen: number;
}) => {
  const [addMember, setAddMember] = useState(false);
  const [loading, setLoading] = useState(false);
  const [password, setPassword] = useState('');
  const { kbList, license, refreshAdminRequest } = useAppSelector(
    state => state.config,
  );

  const {
    control,
    handleSubmit,
    formState: { errors },
    reset,
    watch,
    setValue,
  } = useForm({
    defaultValues: {
      account: '',
      role: 'user' as Role,
      kb_id: '',
      perm: '' as V1KBUserInviteReq['perm'],
    },
  });

  const account = watch('account');
  const watchRole = watch('role');
  const watchKbId = watch('kb_id');

  useEffect(() => {
    if (watchKbId) {
      setValue('perm', ConstsUserKBPermission.UserKBPermissionFullControl);
    }
  }, [watchKbId]);

  const copyUserInfo = ({
    account,
    password,
  }: {
    account: string;
    password: string;
  }) => {
    copyText(`用户名: ${account}\n密码: ${password}`, () => {
      setPassword('');
      reset();
    });
  };

  const onSubmit = handleSubmit(data => {
    setLoading(true);
    const password = generatePassword();
    const onSuccess = () => {
      setPassword(password);
      setAddMember(false);
      refresh();
    };
    postApiV1UserCreate({ account: data.account, password, role: data.role })
      .then(res => {
        if (data.kb_id && data.role === 'user') {
          postApiV1KnowledgeBaseUserInvite({
            kb_id: data.kb_id,
            // @ts-expect-error 类型错误
            user_id: res.id,
            perm: data.perm,
          }).then(() => {
            onSuccess();
            if (location.pathname.startsWith('/setting')) {
              refreshAdminRequest();
            }
          });
        }
        onSuccess();
      })
      .finally(() => {
        setLoading(false);
      });
  });

  const isPro = useMemo(() => {
    return PROFESSION_VERSION_PERMISSION.includes(license.edition!);
  }, [license.edition]);

  return (
    <>
      <Button
        size='small'
        variant='outlined'
        onClick={() => {
          const versionLimit =
            VERSION_MAP[license.edition as keyof typeof VERSION_MAP];
          if (versionLimit && userLen >= versionLimit.max) {
            message.error(versionLimit.message);
            return;
          }
          setAddMember(true);
        }}
      >
        添加新管理员
      </Button>
      <Modal
        title={
          <Stack direction='row' alignItems='center' gap={1}>
            <CheckCircle sx={{ color: 'success.main' }} />
            新用户创建成功
          </Stack>
        }
        open={!!password}
        closable={false}
        cancelText='关闭'
        onCancel={() => {
          setPassword('');
          reset();
        }}
        okText='复制用户信息'
        okButtonProps={{
          sx: { minWidth: '120px' },
        }}
        onOk={() => copyUserInfo({ account, password })}
      >
        <Card sx={{ p: 2, fontSize: 14, bgcolor: 'background.paper3' }}>
          <Stack direction={'row'}>
            <Box sx={{ width: 80 }}>用户名</Box>
            <Box sx={{ fontWeight: 700 }}>{account}</Box>
          </Stack>
          <Stack direction={'row'} sx={{ mt: 1 }}>
            <Box sx={{ width: 80 }}>密码</Box>
            <Box sx={{ fontWeight: 700 }}>{password}</Box>
          </Stack>
        </Card>
      </Modal>
      <Modal
        title='添加新管理员'
        open={addMember}
        onCancel={() => {
          setAddMember(false);
          reset();
        }}
        onOk={onSubmit}
        okButtonProps={{
          loading,
        }}
      >
        <FormItem label='用户名' required>
          <Controller
            control={control}
            name='account'
            rules={{
              required: {
                value: true,
                message: '用户名不能为空',
              },
            }}
            render={({ field }) => (
              <TextField
                {...field}
                fullWidth
                autoFocus
                placeholder='输入用户名'
                error={!!errors.account}
                helperText={errors.account?.message}
              />
            )}
          />
        </FormItem>

        <FormItem label='角色' required sx={{ mt: 2 }}>
          <Controller
            control={control}
            name='role'
            render={({ field }) => (
              <TextField {...field} fullWidth select>
                <MenuItem value='user'>普通管理员</MenuItem>
                <MenuItem value='admin'>超级管理员</MenuItem>
              </TextField>
            )}
          />
        </FormItem>

        {watchRole === 'user' && (
          <>
            <FormItem label='wiki 站' sx={{ mt: 2 }}>
              <Controller
                control={control}
                name='kb_id'
                render={({ field }) => (
                  <Select
                    {...field}
                    fullWidth
                    displayEmpty
                    renderValue={(value: string) =>
                      value ? (
                        kbList?.find(i => i.id === value)?.name || value
                      ) : (
                        <Box sx={{ color: '#9e9fa3' }}>请选择</Box>
                      )
                    }
                    sx={{ height: 52 }}
                  >
                    {kbList?.map(item => (
                      <MenuItem key={item.id} value={item.id}>
                        {item.name}
                      </MenuItem>
                    ))}
                  </Select>
                )}
              />
            </FormItem>
            <FormItem label='权限' sx={{ mt: 2 }}>
              <Controller
                control={control}
                name='perm'
                render={({ field }) => (
                  <Select
                    {...field}
                    fullWidth
                    displayEmpty
                    sx={{ height: 52 }}
                    MenuProps={{
                      sx: {
                        '.Mui-disabled': {
                          opacity: '1 !important',
                          color: 'text.disabled',
                        },
                      },
                    }}
                    renderValue={(value: V1KBUserInviteReq['perm']) => {
                      return value ? (
                        PERM_MAP[value]
                      ) : (
                        <Box sx={{ color: '#9e9fa3' }}>请选择</Box>
                      );
                    }}
                  >
                    <MenuItem
                      value={ConstsUserKBPermission.UserKBPermissionFullControl}
                    >
                      完全控制
                    </MenuItem>

                    <MenuItem
                      value={ConstsUserKBPermission.UserKBPermissionDocManage}
                      disabled={!isPro}
                    >
                      文档管理{' '}
                      <VersionCanUse
                        permission={PROFESSION_VERSION_PERMISSION}
                      />
                    </MenuItem>

                    <MenuItem
                      value={ConstsUserKBPermission.UserKBPermissionDataOperate}
                      disabled={!isPro}
                    >
                      数据运营{' '}
                      <VersionCanUse
                        permission={PROFESSION_VERSION_PERMISSION}
                      />
                    </MenuItem>
                  </Select>
                )}
              />
            </FormItem>
          </>
        )}
      </Modal>
    </>
  );
};

export default MemberAdd;
