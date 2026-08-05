import { createTheme, ThemeProvider } from '@mui/material/styles';
import { CssBaseline } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { type PropsWithChildren, useEffect, useState } from 'react';
import { Provider, useDispatch } from 'react-redux';
import { useLocation, useNavigate } from 'react-router-dom';
import { setUnauthorizedHandler } from '@/api/client';
import { clearStoredSession } from '@/features/auth/session';
import { createAppStore, type AppDispatch } from '@/store';
import { clearAuth } from '@/store/authSlice';

const theme = createTheme({
  palette: {
    mode: 'light',
    primary: { main: '#5b5bd6' },
    background: { default: '#f6f6f8', paper: '#ffffff' },
    text: { primary: '#202124', secondary: '#6b6f76' },
  },
  shape: { borderRadius: 10 },
  typography: {
    fontFamily: 'Gilroy, Inter, "PingFang SC", "Microsoft YaHei", sans-serif',
  },
});

function UnauthorizedBridge() {
  const dispatch = useDispatch<AppDispatch>();
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    setUnauthorizedHandler(() => {
      clearStoredSession();
      dispatch(clearAuth());
      if (location.pathname !== '/login') {
        const redirect = `${location.pathname}${location.search}${location.hash}`;
        navigate(`/login?redirect=${encodeURIComponent(redirect)}`, { replace: true });
      }
    });
    return () => setUnauthorizedHandler(undefined);
  }, [dispatch, location.hash, location.pathname, location.search, navigate]);

  return null;
}

export function AppProviders({ children }: PropsWithChildren) {
  const [store] = useState(createAppStore);
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: { retry: false, refetchOnWindowFocus: false },
          mutations: { retry: false },
        },
      }),
  );

  return (
    <Provider store={store}>
      <QueryClientProvider client={queryClient}>
        <ThemeProvider theme={theme}>
          <CssBaseline />
          <UnauthorizedBridge />
          {children}
        </ThemeProvider>
      </QueryClientProvider>
    </Provider>
  );
}
