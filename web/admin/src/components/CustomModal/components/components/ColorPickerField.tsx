import React from 'react';
import { Box, InputAdornment, Popover, TextField } from '@mui/material';
import type { SxProps } from '@mui/material/styles';
import { ColorPicker, useColor, ColorService } from 'react-color-palette';
// @ts-expect-error ignore
import 'react-color-palette/css';

type ColorPickerFieldProps = {
  label?: string;
  value?: string;
  onChange?: (hex: string) => void;
  width?: number;
  placeholder?: string;
  sx?: SxProps;
};

const ColorPickerField: React.FC<ColorPickerFieldProps> = ({
  label,
  value = '#000000',
  onChange,
  width = 320,
  placeholder = '请输入',
  sx,
}) => {
  const [anchorEl, setAnchorEl] = React.useState<HTMLElement | null>(null);
  const [color, setColor] = useColor(value || '#000000');

  React.useEffect(() => {
    if (value && value !== color.hex) {
      setColor(ColorService.convert('hex', value));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);

  const open = Boolean(anchorEl);
  const handleOpen = (e: React.MouseEvent<HTMLElement>) =>
    setAnchorEl(e.currentTarget);
  const handleClose = () => setAnchorEl(null);

  return (
    <>
      <TextField
        label={label}
        onClick={handleOpen}
        slotProps={{
          inputLabel: {
            shrink: true,
          },
          input: {
            readOnly: true,
            startAdornment: (
              <InputAdornment position='start'>
                <Box
                  sx={{
                    width: 20,
                    height: 20,
                    borderRadius: '4px',
                    backgroundColor: value,
                  }}
                />
              </InputAdornment>
            ),
          },
        }}
        sx={sx}
        value={value}
        placeholder={placeholder}
      />
      <Popover
        open={open}
        anchorEl={anchorEl}
        onClose={handleClose}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        transformOrigin={{ vertical: 'top', horizontal: 'left' }}
      >
        <Box sx={{ p: 1, width }}>
          <ColorPicker
            color={color}
            onChange={c => {
              setColor(c);
              onChange?.(c.hex);
            }}
          />
        </Box>
      </Popover>
    </>
  );
};

export default ColorPickerField;
