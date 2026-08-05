import { Alert, Button, Divider, Paper, Stack, TextField, Typography } from '@mui/material';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { type FormEvent, useEffect, useState } from 'react';
import { queryKeys } from '@/api/queryKeys';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { updateStoredUser } from '@/features/auth/session';
import { useAppDispatch } from '@/store';
import { updateAuthUser } from '@/store/authSlice';
import { changePassword, getCurrentUser, updateCurrentUser } from '../api';
import type { User } from '../types';

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export function ProfilePage() {
  const queryClient = useQueryClient();
  const dispatch = useAppDispatch();
  const [nickname, setNickname] = useState('');
  const [email, setEmail] = useState('');
  const [avatarUrl, setAvatarUrl] = useState('');
  const [bio, setBio] = useState('');
  const [profileMessage, setProfileMessage] = useState('');
  const [profileValidation, setProfileValidation] = useState('');
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [passwordMessage, setPasswordMessage] = useState('');
  const [passwordValidation, setPasswordValidation] = useState('');

  const userQuery = useQuery({ queryKey: queryKeys.currentUser, queryFn: getCurrentUser });

  useEffect(() => {
    if (!userQuery.data) return;
    setNickname(userQuery.data.nickname);
    setEmail(userQuery.data.email);
    setAvatarUrl(userQuery.data.avatar_url ?? '');
    setBio(userQuery.data.bio ?? '');
  }, [userQuery.data]);

  const profileMutation = useMutation({
    mutationFn: updateCurrentUser,
    onSuccess: (user: User) => {
      queryClient.setQueryData(queryKeys.currentUser, user);
      updateStoredUser(user);
      dispatch(updateAuthUser(user));
      setProfileMessage('资料已更新');
    },
  });

  const passwordMutation = useMutation({
    mutationFn: changePassword,
    onSuccess: () => {
      setOldPassword('');
      setNewPassword('');
      setPasswordMessage('密码已修改');
    },
  });

  const submitProfile = (event: FormEvent) => {
    event.preventDefault();
    setProfileMessage('');
    if (!emailPattern.test(email)) {
      setProfileValidation('请输入有效的邮箱地址');
      return;
    }
    setProfileValidation('');
    profileMutation.mutate({
      nickname,
      email,
      ...(avatarUrl ? { avatar_url: avatarUrl } : {}),
      bio,
    });
  };

  const submitPassword = (event: FormEvent) => {
    event.preventDefault();
    setPasswordMessage('');
    if (newPassword.length < 12) {
      setPasswordValidation('新密码至少需要 12 个字符');
      return;
    }
    setPasswordValidation('');
    passwordMutation.mutate({ old_password: oldPassword, new_password: newPassword });
  };

  if (userQuery.isPending) return <LoadingState label="正在加载个人资料" />;
  if (userQuery.error) {
    return <ErrorState error={userQuery.error as Error} onRetry={() => void userQuery.refetch()} />;
  }

  return (
    <Stack spacing={3} maxWidth={760}>
      <Paper component="form" variant="outlined" sx={{ p: 3 }} onSubmit={submitProfile} noValidate>
        <Stack spacing={2.5}>
          <Typography component="h2" variant="h5" fontWeight={750}>个人资料</Typography>
          {profileMessage && <Alert severity="success">{profileMessage}</Alert>}
          {profileMutation.error && <ErrorState error={profileMutation.error as Error} />}
          <TextField label="用户名" value={userQuery.data.username} disabled />
          <TextField label="昵称" value={nickname} onChange={(event) => setNickname(event.target.value)} />
          <TextField
            label="邮箱"
            type="email"
            value={email}
            error={Boolean(profileValidation)}
            helperText={profileValidation}
            onChange={(event) => setEmail(event.target.value)}
          />
          <TextField label="头像 URL" value={avatarUrl} onChange={(event) => setAvatarUrl(event.target.value)} />
          <TextField label="个人简介" value={bio} multiline minRows={3} onChange={(event) => setBio(event.target.value)} />
          <Button type="submit" variant="contained" disabled={profileMutation.isPending}>保存资料</Button>
        </Stack>
      </Paper>

      <Paper component="form" variant="outlined" sx={{ p: 3 }} onSubmit={submitPassword} noValidate>
        <Stack spacing={2.5}>
          <Typography component="h2" variant="h5" fontWeight={750}>修改密码</Typography>
          <Divider />
          {passwordMessage && <Alert severity="success">{passwordMessage}</Alert>}
          {passwordMutation.error && <ErrorState error={passwordMutation.error as Error} />}
          <TextField label="当前密码" type="password" value={oldPassword} onChange={(event) => setOldPassword(event.target.value)} />
          <TextField
            label="新密码"
            type="password"
            value={newPassword}
            error={Boolean(passwordValidation)}
            helperText={passwordValidation || '至少 12 个字符'}
            onChange={(event) => setNewPassword(event.target.value)}
          />
          <Button type="submit" variant="outlined" disabled={passwordMutation.isPending}>修改密码</Button>
        </Stack>
      </Paper>
    </Stack>
  );
}
