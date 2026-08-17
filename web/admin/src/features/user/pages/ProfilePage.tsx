import AccountCircleOutlined from '@mui/icons-material/AccountCircleOutlined';
import AccessTimeOutlined from '@mui/icons-material/AccessTimeOutlined';
import CheckCircleOutlined from '@mui/icons-material/CheckCircleOutlined';
import DescriptionOutlined from '@mui/icons-material/DescriptionOutlined';
import MenuBookOutlined from '@mui/icons-material/MenuBookOutlined';
import UploadOutlined from '@mui/icons-material/UploadOutlined';
import { Alert, Box, Button, Chip, InputAdornment, Paper, Stack, TextField, Typography } from '@mui/material';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { type FormEvent, useEffect, useRef, useState } from 'react';
import { queryKeys } from '@/api/queryKeys';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { updateStoredUser } from '@/features/auth/session';
import { listKnowledgeBases } from '@/features/knowledge-base/api';
import { useAppDispatch } from '@/store';
import { updateAuthUser } from '@/store/authSlice';
import { getCurrentUser, updateCurrentUser, uploadAvatar } from '../api';
import type { User } from '../types';

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const maxAvatarFileSize = 5 * 1024 * 1024;
const acceptedAvatarTypes = new Set(['image/jpeg', 'image/png', 'image/gif', 'image/webp']);

export function ProfilePage() {
  const queryClient = useQueryClient();
  const dispatch = useAppDispatch();
  const avatarInputRef = useRef<HTMLInputElement>(null);
  const previewObjectUrlRef = useRef<string | null>(null);
  const [nickname, setNickname] = useState('');
  const [email, setEmail] = useState('');
  const [avatarPreview, setAvatarPreview] = useState('');
  const [avatarValidation, setAvatarValidation] = useState('');
  const [bio, setBio] = useState('');
  const [profileMessage, setProfileMessage] = useState('');
  const [profileValidation, setProfileValidation] = useState('');

  const userQuery = useQuery({ queryKey: queryKeys.currentUser, queryFn: getCurrentUser });
  const knowledgeBasesQuery = useQuery({
    queryKey: [...queryKeys.knowledgeBases, 'profile-summary'],
    queryFn: () => listKnowledgeBases({ page: 1, page_size: 100, sort: 'updated_at_desc' }),
  });

  useEffect(() => {
    if (!userQuery.data) return;
    setNickname(userQuery.data.nickname);
    setEmail(userQuery.data.email);
    if (!previewObjectUrlRef.current) setAvatarPreview(userQuery.data.avatar_url ?? '');
    setBio(userQuery.data.bio ?? '');
  }, [userQuery.data]);

  useEffect(() => () => {
    if (previewObjectUrlRef.current) URL.revokeObjectURL(previewObjectUrlRef.current);
  }, []);

  const syncUser = (user: User) => {
    queryClient.setQueryData(queryKeys.currentUser, user);
    updateStoredUser(user);
    dispatch(updateAuthUser(user));
  };

  const profileMutation = useMutation({
    mutationFn: updateCurrentUser,
    onSuccess: (user: User) => {
      syncUser(user);
      setProfileMessage('资料更新成功！');
    },
  });

  const avatarMutation = useMutation({
    mutationFn: uploadAvatar,
    onSuccess: (user: User) => {
      if (previewObjectUrlRef.current) {
        URL.revokeObjectURL(previewObjectUrlRef.current);
        previewObjectUrlRef.current = null;
      }
      setAvatarPreview(user.avatar_url ?? '');
      syncUser(user);
      setAvatarValidation('');
      setProfileMessage('头像更新成功！');
    },
    onError: () => {
      if (previewObjectUrlRef.current) {
        URL.revokeObjectURL(previewObjectUrlRef.current);
        previewObjectUrlRef.current = null;
      }
      setAvatarPreview(userQuery.data?.avatar_url ?? '');
    },
  });

  const selectAvatar = (file: File | undefined) => {
    if (!file) return;
    setProfileMessage('');
    setAvatarValidation('');
    if ((file.type && !acceptedAvatarTypes.has(file.type)) || file.size <= 0) {
      setAvatarValidation('请选择 JPG、PNG、GIF 或 WebP 图片');
      return;
    }
    if (file.size > maxAvatarFileSize) {
      setAvatarValidation('头像图片不能超过 5 MB');
      return;
    }
    if (previewObjectUrlRef.current) URL.revokeObjectURL(previewObjectUrlRef.current);
    const previewUrl = URL.createObjectURL(file);
    previewObjectUrlRef.current = previewUrl;
    setAvatarPreview(previewUrl);
    avatarMutation.mutate(file);
  };

  const submitProfile = (event: FormEvent) => {
    event.preventDefault();
    setProfileMessage('');
    if (!nickname.trim()) {
      setProfileValidation('昵称不能为空');
      return;
    }
    if (nickname.trim().length > 64) {
      setProfileValidation('昵称不能超过 64 个字符');
      return;
    }
    if (!emailPattern.test(email)) {
      setProfileValidation('请输入有效的邮箱地址');
      return;
    }
    if (bio.length > 200) {
      setProfileValidation('个人简介不能超过 200 个字符');
      return;
    }
    setProfileValidation('');
    profileMutation.mutate({
      nickname: nickname.trim(),
      email: email.trim(),
      bio: bio.trim(),
    });
  };

  if (userQuery.isPending) return <LoadingState label="正在加载个人资料" />;
  if (userQuery.error) return <ErrorState error={userQuery.error as Error} onRetry={() => void userQuery.refetch()} />;

  const user = userQuery.data;
  const documentCount = (knowledgeBasesQuery.data?.items ?? []).reduce((total, kb) => total + kb.document_count, 0);
  const avatarInitial = (user.nickname || user.username || 'A').trim().charAt(0).toUpperCase();
  return (
    <Stack spacing={2.4} sx={{ width: '100%', maxWidth: 1220, mx: 'auto' }}>
      <Stack direction="row" alignItems="flex-start" spacing={2}>
        <Box sx={{ flexGrow: 1 }}>
          <Typography component="h2" sx={{ color: '#111c3a', fontSize: { xs: 27, md: 31 }, fontWeight: 700, lineHeight: 1.2 }}>个人资料设置</Typography>
          <Typography sx={{ color: '#66728c', fontSize: 14, mt: 0.75 }}>管理您的个人信息和账户安全设置，确保账户信息准确完整。</Typography>
        </Box>
        {profileMessage && <Alert severity="success" onClose={() => setProfileMessage('')} sx={{ minWidth: 300, border: '1px solid #d7ebdd', bgcolor: '#fbfffc', boxShadow: '0 8px 24px rgba(31,90,48,.08)' }}>{profileMessage}</Alert>}
      </Stack>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', lg: '400px minmax(0, 800px)' }, gap: 2, alignItems: 'stretch', justifyContent: 'center' }}>
        <Paper variant="outlined" sx={{ p: 3, borderRadius: 3.5, borderColor: '#e1e5ed', boxShadow: '0 8px 26px rgba(31,45,90,.035)' }}>
          <Typography sx={{ color: '#172343', fontSize: 18, fontWeight: 700 }}>个人信息概览</Typography>
          <Stack alignItems="center" sx={{ mt: 3.5 }}>
            <Box sx={{ position: 'relative', width: 118, height: 118, borderRadius: '50%', display: 'grid', placeItems: 'center', overflow: 'hidden', color: '#fff', fontSize: 44, background: 'linear-gradient(145deg,#564bf3,#6f72f4)', border: '3px solid #fff', boxShadow: '0 12px 32px rgba(78,72,222,.28)' }}>
              {avatarInitial}
              {avatarPreview && <Box component="img" src={avatarPreview} alt={`${user.nickname} 的头像`} sx={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover' }} onError={(event) => { event.currentTarget.style.display = 'none'; }} />}
            </Box>
            <Box component="input" ref={avatarInputRef} type="file" accept="image/jpeg,image/png,image/gif,image/webp" hidden onChange={(event) => { selectAvatar(event.currentTarget.files?.[0]); event.currentTarget.value = ''; }} />
            <Button variant="outlined" startIcon={<UploadOutlined />} onClick={() => avatarInputRef.current?.click()} disabled={avatarMutation.isPending} sx={{ mt: 1.7, borderRadius: 2 }}>{avatarMutation.isPending ? '正在上传…' : '上传头像'}</Button>
            {avatarValidation && <Alert severity="warning" sx={{ width: '100%', mt: 1.4 }}>{avatarValidation}</Alert>}
            {avatarMutation.error && <Alert severity="error" sx={{ width: '100%', mt: 1.4 }}>头像上传失败，请稍后重试。</Alert>}
            <Typography sx={{ color: '#172343', fontSize: 21, fontWeight: 700, mt: 1.7 }}>{user.nickname || user.username}</Typography>
            <Chip size="small" icon={<CheckCircleOutlined />} label="已验证" sx={{ mt: 0.6, bgcolor: '#e8f7ec', color: '#29934f' }} />
            <Typography sx={{ color: '#526078', fontSize: 14, mt: 0.7 }}>{user.email}</Typography>
            <Typography sx={{ maxWidth: 300, color: '#748097', fontSize: 13, lineHeight: 1.7, textAlign: 'center', mt: 2 }}>{user.bio || '完善个人简介，让团队成员更好地了解您。'}</Typography>
          </Stack>

          <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 1, mt: 2.7 }}>
            {[
              { icon: <MenuBookOutlined />, label: '已创建知识库', value: knowledgeBasesQuery.data?.total ?? 0 },
              { icon: <DescriptionOutlined />, label: '文档数', value: documentCount },
              { icon: <AccessTimeOutlined />, label: '最近活跃', value: '今天' },
            ].map((item) => <Box key={item.label} sx={{ minHeight: 112, p: 1.5, border: '1px solid #e2e6ed', borderRadius: 2.5 }}><Box sx={{ width: 36, height: 36, borderRadius: 1.8, display: 'grid', placeItems: 'center', bgcolor: '#eef0ff', color: '#5c61ec', '& svg': { fontSize: 20 } }}>{item.icon}</Box><Typography sx={{ color: '#79859b', fontSize: 11.5, mt: 1 }}>{item.label}</Typography><Typography sx={{ color: '#172343', fontSize: 20, fontWeight: 700 }}>{item.value}</Typography></Box>)}
          </Box>
          <Stack direction="row" alignItems="center" spacing={1} sx={{ mt: 2, pt: 1.8, borderTop: '1px solid #e7eaf0' }}><AccessTimeOutlined sx={{ color: '#8793a8', fontSize: 17 }} /><Typography sx={{ color: '#7b879c', fontSize: 12 }}>资料信息已同步至当前账户</Typography></Stack>
        </Paper>

        <Stack sx={{ height: '100%' }}>
          <Paper component="form" variant="outlined" sx={{ height: '100%', p: 2.7, display: 'flex', flexDirection: 'column', borderRadius: 3.5, borderColor: '#e1e5ed', boxShadow: '0 8px 26px rgba(31,45,90,.035)' }} onSubmit={submitProfile} noValidate>
            <Stack direction="row" spacing={1.2} alignItems="center" mb={2}><Box sx={{ width: 36, height: 36, borderRadius: '50%', display: 'grid', placeItems: 'center', bgcolor: '#eef0ff', color: '#5a61ed' }}><AccountCircleOutlined sx={{ fontSize: 21 }} /></Box><Typography sx={{ color: '#172343', fontSize: 16, fontWeight: 700 }}>基本信息</Typography></Stack>
            {profileValidation && <Alert severity="warning" sx={{ mb: 1.5 }}>{profileValidation}</Alert>}
            {profileMutation.error && <Alert severity="error" sx={{ mb: 1.5 }}>资料保存失败，请检查输入后重试。</Alert>}
            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' }, gap: 1.6 }}>
              <TextField label="用户名" value={user.username} disabled size="small" />
              <TextField label="昵称" value={nickname} onChange={(event) => setNickname(event.target.value)} size="small" inputProps={{ maxLength: 64 }} />
              <TextField label="邮箱" type="email" value={email} onChange={(event) => setEmail(event.target.value)} size="small" sx={{ gridColumn: { sm: '1 / -1' } }} InputProps={{ endAdornment: <InputAdornment position="end"><Chip size="small" icon={<CheckCircleOutlined />} label="已验证" sx={{ bgcolor: '#e8f7ec', color: '#29934f' }} /></InputAdornment> }} />
              <TextField label="个人简介" value={bio} multiline minRows={4} onChange={(event) => setBio(event.target.value)} inputProps={{ maxLength: 200 }} helperText={`${bio.length} / 200`} FormHelperTextProps={{ sx: { textAlign: 'right' } }} sx={{ gridColumn: { sm: '1 / -1' } }} />
            </Box>
            <Stack direction="row" justifyContent="flex-end" sx={{ mt: 'auto', pt: 2.2 }}>
              <Button type="submit" variant="contained" disabled={profileMutation.isPending} sx={{ width: 168, minHeight: 40, borderRadius: 2, background: 'linear-gradient(135deg,#3f63f0,#7944ed)', boxShadow: '0 6px 16px rgba(78,78,224,.22)', '&:hover': { background: 'linear-gradient(135deg,#3558e6,#6e38df)', boxShadow: '0 8px 20px rgba(78,78,224,.28)' } }}>{profileMutation.isPending ? '正在保存…' : '保存更改'}</Button>
            </Stack>
          </Paper>
        </Stack>
      </Box>
    </Stack>
  );
}
