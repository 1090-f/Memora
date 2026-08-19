import { Alert, Snackbar, type AlertColor } from '@mui/material';

export function ActionNotice({ message, severity = 'success', onClose, duration }: {
  message: string;
  severity?: AlertColor;
  onClose: () => void;
  duration?: number;
}) {
  return (
    <Snackbar
      key={`${severity}:${message}`}
      open={message !== ''}
      autoHideDuration={duration ?? (severity === 'success' ? 3500 : 6000)}
      anchorOrigin={{ vertical: 'top', horizontal: 'right' }}
      onClose={(_, reason) => { if (reason !== 'clickaway') onClose(); }}
      sx={{ mt: 6.5, mr: { xs: 0, sm: 1.5 } }}
    >
      <Alert
        severity={severity}
        variant="standard"
        onClose={onClose}
        sx={{
          width: { xs: 'calc(100vw - 32px)', sm: 'auto' },
          minWidth: { sm: 320 },
          maxWidth: 460,
          alignItems: 'center',
          border: '1px solid',
          borderColor: severity === 'success' ? '#cce8d5' : severity === 'error' ? '#f0c7c7' : '#ead9aa',
          bgcolor: '#fff',
          boxShadow: '0 12px 32px rgba(25, 38, 70, .16)',
        }}
      >
        {message}
      </Alert>
    </Snackbar>
  );
}
