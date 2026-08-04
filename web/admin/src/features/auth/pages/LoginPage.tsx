import { Alert, Box, Button, Paper, Stack, TextField, Typography } from '@mui/material';
import { type FormEvent, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { AppError } from '@/api/errors';
import { RequestIdCopy } from '@/components/shared/RequestIdCopy';
import { useAppDispatch } from '@/store';
import { setAuthSession } from '@/store/authSlice';
import { login } from '../api';
import { saveLoginSession } from '../session';

export function LoginPage() {
  const [account, setAccount] = useState('');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<AppError | null>(null);
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!account.trim() || !password) return;
    setSubmitting(true);
    setError(null);
    try {
      const response = await login({ account: account.trim(), password });
      saveLoginSession(response);
      dispatch(setAuthSession(response.user));
      const requested = searchParams.get('redirect');
      const destination = requested?.startsWith('/') && !requested.startsWith('//')
        ? requested
        : '/knowledge-bases';
      navigate(destination, { replace: true });
    } catch (reason) {
      setError(
        reason instanceof AppError
          ? reason
          : new AppError('UNKNOWN', '登录失败', 0, undefined, ''),
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Box sx={{ minHeight: '100vh', display: 'grid', placeItems: 'center', p: 3 }}>
      <Paper elevation={0} sx={{ width: 'min(440px, 100%)', p: 5, border: '1px solid #e5e6ea' }}>
        <Stack component="form" spacing={2.5} onSubmit={submit} noValidate>
          <Typography component="h1" variant="h4" fontWeight={800}>登录 Memora</Typography>
          <Typography color="text.secondary">进入你的智能知识库与 Agent 工作台。</Typography>
          {error && (
            <Alert severity="error">
              {error.message}
              {error.requestId && <RequestIdCopy requestId={error.requestId} />}
            </Alert>
          )}
          <TextField
            label="账号"
            value={account}
            onChange={(event) => setAccount(event.target.value)}
            autoComplete="username"
            autoFocus
          />
          <TextField
            label="密码"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            autoComplete="current-password"
          />
          <Button type="submit" variant="contained" size="large" disabled={submitting}>
            {submitting ? '正在登录…' : '登录'}
          </Button>
        </Stack>
      </Paper>
    </Box>
  );
}
