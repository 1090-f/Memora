import React from 'react';
import SvgIcon, { SvgIconProps } from '@mui/material/SvgIcon';

const IconWenzi = (props: SvgIconProps) => (
  <SvgIcon
    xmlns='http://www.w3.org/2000/svg'
    viewBox='0 0 1024 1024'
    {...props}
  >
    <path d='M563.2 281.6V870.4a51.2 51.2 0 0 1-102.4 0V281.6H179.2a51.2 51.2 0 1 1 0-102.4h665.6a51.2 51.2 0 0 1 0 102.4H563.2z'></path>
  </SvgIcon>
);

IconWenzi.displayName = 'icon-wenzi';

export default IconWenzi;
