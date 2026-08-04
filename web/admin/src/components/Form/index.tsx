import { styled, FormLabel, Box, SxProps } from '@mui/material';

export const StyledFormLabel = styled(FormLabel)(({ theme }) => ({
  display: 'block',
  color: theme.palette.text.primary,
  fontSize: 14,
  fontWeight: 400,
  marginBottom: theme.spacing(1),
  [theme.breakpoints.down('sm')]: {
    fontSize: 14,
  },
}));

export const FormItem = ({
  label,
  children,
  required,
  sx,
}: {
  label: string | React.ReactNode;
  children: React.ReactNode;
  required?: boolean;
  sx?: SxProps;
}) => {
  return (
    <Box sx={sx}>
      <StyledFormLabel required={required}>{label}</StyledFormLabel>
      {children}
    </Box>
  );
};
