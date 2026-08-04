import { Box, CssBaseline, Typography } from '@mui/material';
import { createTheme, ThemeProvider } from '@mui/material/styles';

const theme = createTheme({
  palette: {
    mode: 'light',
    primary: { main: '#5b5bd6' },
    background: { default: '#f7f7f8', paper: '#ffffff' },
  },
  shape: { borderRadius: 10 },
  typography: {
    fontFamily: 'Gilroy, Inter, "PingFang SC", "Microsoft YaHei", sans-serif',
  },
});

function App() {
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <Box component="main" sx={{ minHeight: '100vh', p: 4 }}>
        <Typography component="h1" variant="h4" fontWeight={700}>
          Memora
        </Typography>
      </Box>
    </ThemeProvider>
  );
}

export default App;
