import Card from '@/components/Card';
import { ConstsLicenseEdition } from '@/request/types';
import InfoIcon from '@mui/icons-material/Info';
import Visibility from '@mui/icons-material/Visibility';
import VisibilityOff from '@mui/icons-material/VisibilityOff';
import {
  Button,
  IconButton,
  InputAdornment,
  Stack,
  styled,
  SxProps,
  TextField,
  TextFieldProps,
  Tooltip,
} from '@mui/material';
import { createContext, useContext, useState } from 'react';
import VersionMask from '@/components/VersionMask';

export const SecretTextField = (props: TextFieldProps) => {
  const [show, setShow] = useState(false);
  return (
    <TextField
      {...props}
      type={show ? 'text' : 'password'}
      slotProps={{
        ...props.slotProps,
        input: {
          ...((props.slotProps?.input ?? {}) as Record<string, unknown>),
          endAdornment: (
            <InputAdornment position='end'>
              <IconButton
                onClick={() => setShow(prev => !prev)}
                edge='end'
                size='small'
              >
                {show ? <VisibilityOff /> : <Visibility />}
              </IconButton>
            </InputAdornment>
          ),
        },
      }}
    />
  );
};

const StyledForm = styled('form')<{ gap?: number | string }>(
  ({ theme, gap = 2 }) => ({
    display: 'flex',
    flexDirection: 'column',
    gap: typeof gap === 'number' ? theme.spacing(gap) : gap,
  }),
);

const StyledFormLabelWrapper = styled('div')<{
  vertical?: boolean;
  labelWidth?: number;
}>(({ vertical, theme, labelWidth }) => ({
  width: vertical ? '100%' : labelWidth || 156,
  flexShrink: 0,
  display: 'flex',
  alignItems: 'center',
  marginBottom: vertical ? theme.spacing(1) : 0,
}));

const StyledFormLabel = styled('span')<{ required?: boolean }>(
  ({ theme, required }) => ({
    position: 'relative',
    fontSize: 14,
    '&::before': required && {
      content: '"*"',
      fontSize: 16,
      display: 'inline-block',
      position: 'absolute',
      right: -8,
      top: -2,
      color: theme.palette.error.main,
    },
  }),
);

export const StyledFormItem = styled('div')<{ vertical?: boolean }>(
  ({ theme, vertical }) => ({
    position: 'relative',
    display: 'flex',
    alignItems: vertical ? 'flex-start' : 'center',
    flexDirection: vertical ? 'column' : 'row',
    gap: vertical ? 0 : theme.spacing(2),
  }),
);

const FormContext = createContext<{
  vertical?: boolean;
  labelWidth?: number;
}>({});

export const Form = ({
  children,
  vertical,
  labelWidth,
  gap,
}: {
  children: React.ReactNode;
  vertical?: boolean;
  labelWidth?: number;
  gap?: number | string;
}) => {
  return (
    <StyledForm gap={gap}>
      <FormContext.Provider value={{ vertical, labelWidth }}>
        {children}
      </FormContext.Provider>
    </StyledForm>
  );
};

export const FormItem = ({
  label,
  children,
  required,
  vertical = false,
  labelWidth,
  tooltip,
  extra,
  sx,
  labelSx,
  permission,
}: {
  label?: string | React.ReactNode;
  children?: React.ReactNode;
  required?: boolean;
  vertical?: boolean;
  labelWidth?: number;
  tooltip?: React.ReactNode;
  extra?: React.ReactNode;
  sx?: SxProps;
  labelSx?: SxProps;
  permission?: number[];
}) => {
  const { vertical: verticalContext, labelWidth: labelWidthContext } =
    useContext(FormContext);

  return (
    <VersionMask permission={permission}>
      <StyledFormItem vertical={vertical || verticalContext} sx={sx}>
        <StyledFormLabelWrapper
          vertical={vertical || verticalContext}
          labelWidth={labelWidth || labelWidthContext}
          sx={labelSx}
        >
          <Stack direction='row' alignItems='center' flex={1}>
            <StyledFormLabel required={required}>{label}</StyledFormLabel>
            {tooltip && typeof tooltip === 'string' ? (
              <Tooltip title={tooltip} placement='top' arrow>
                <InfoIcon
                  sx={{ color: 'text.secondary', fontSize: 14, ml: 1 }}
                />
              </Tooltip>
            ) : (
              tooltip
            )}
          </Stack>

          {extra}
        </StyledFormLabelWrapper>
        {children}
      </StyledFormItem>
    </VersionMask>
  );
};

const StyleSettingCardTitle = styled('div')(({ theme }) => ({
  fontWeight: 'bold',
  padding: theme.spacing(2, 1.5),
  backgroundColor: theme.palette.background.paper3,
}));

export const SettingCard = ({
  children,
  title,
}: {
  children: React.ReactNode;
  title: string;
}) => {
  return (
    <Card sx={{ pb: 2 }}>
      <StyleSettingCardTitle>{title}</StyleSettingCardTitle>
      {children}
    </Card>
  );
};

const StyledSettingCardItem = styled('div')(({ theme }) => ({
  position: 'relative',
  '&:not(:last-child)': {
    borderBottom: `1px solid ${theme.palette.divider}`,
    paddingBottom: theme.spacing(4),
  },
}));

const StyledSettingCardItemTitleWrapper = styled('div')(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  margin: theme.spacing(2),
  height: 32,
  fontWeight: 'bold',
}));

const StyledSettingCardItemTitle = styled('div')(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  '&::before': {
    content: '""',
    width: 4,
    height: 12,
    backgroundColor: theme.palette.common.black,
    borderRadius: '2px',
    marginRight: theme.spacing(1),
  },
}));

const StyledSettingCardItemContent = styled('div')(({ theme }) => ({
  display: 'flex',
  flexDirection: 'column',
  gap: theme.spacing(2),
  padding: theme.spacing(0, 2),
}));

const StyledSettingCardItemTitleMore = styled('a')(({ theme }) => ({
  marginLeft: theme.spacing(1),
  fontSize: 14,
  textDecoration: 'none',
  fontWeight: 'normal',
  '&:hover': {
    fontWeight: 'bold',
  },
}));

type SettingCardItemMore =
  | React.ReactNode
  | {
      type: 'link';
      href: string;
      target?: string;
      text?: string;
    };

export const SettingCardItem = ({
  children,
  title,
  isEdit,
  onSubmit,
  extra,
  more,
  sx,
  permission = [
    ConstsLicenseEdition.LicenseEditionFree,
    ConstsLicenseEdition.LicenseEditionProfession,
    ConstsLicenseEdition.LicenseEditionBusiness,
    ConstsLicenseEdition.LicenseEditionEnterprise,
  ],
}: {
  children?: React.ReactNode;
  title?: React.ReactNode;
  isEdit?: boolean;
  onSubmit?: () => void;
  extra?: React.ReactNode;
  more?: SettingCardItemMore;
  sx?: SxProps;
  permission?: number[];
}) => {
  const renderMore = (more: SettingCardItemMore) => {
    if (more && typeof more === 'object' && 'type' in more) {
      const linkMore = more as {
        type: 'link';
        href: string;
        target?: string;
        text?: string;
      };
      if (linkMore.type === 'link') {
        // 处理链接类型
        return (
          <StyledSettingCardItemTitleMore
            href={linkMore.href}
            target={linkMore.target}
          >
            {linkMore.text ?? '更多'}
          </StyledSettingCardItemTitleMore>
        );
      }
      return more as React.ReactNode;
    } else {
      return more;
    }
  };

  return (
    <VersionMask permission={permission}>
      <StyledSettingCardItem sx={sx}>
        <StyledSettingCardItemTitleWrapper>
          <StyledSettingCardItemTitle>
            {title} {renderMore(more)}
          </StyledSettingCardItemTitle>
          {isEdit && (
            <Button variant='contained' size='small' onClick={onSubmit}>
              保存
            </Button>
          )}
          {extra}
        </StyledSettingCardItemTitleWrapper>
        <StyledSettingCardItemContent>{children}</StyledSettingCardItemContent>
      </StyledSettingCardItem>
    </VersionMask>
  );
};
