import { createTheme, ThemeProvider } from '@mui/material/styles';
import { CssBaseline } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { type PropsWithChildren, useState } from 'react';
import { Provider } from 'react-redux';
import { createAppStore } from '@/store';

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
          {children}
        </ThemeProvider>
      </QueryClientProvider>
    </Provider>
  );
}
