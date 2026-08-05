import React from 'react';
import SvgIcon, { SvgIconProps } from '@mui/material/SvgIcon';

const IconPageview1 = (props: SvgIconProps) => (
  <SvgIcon
    xmlns='http://www.w3.org/2000/svg'
    viewBox='0 0 1024 1024'
    {...props}
  >
    <path
      d='M512 192c206.752 0 419.872 254.016 446.496 320-26.56 66.016-239.744 320-446.496 320-207.456 0-420.416-254.176-446.496-320 26.24-65.792 239.2-320 446.496-320zM512 128c-255.808 0-512 319.808-512 384 0 64.064 255.104 384 512 384 256.32 0 512-320.384 512-384 0-64-256.864-384-512-384l0 0z'
      fill='#444444'
    ></path>
    <path
      d='M512 352c-88.416 0-160 71.648-160 160 0 88.384 71.584 160 160 160 88.448 0 160-71.616 160-160 0-88.352-71.552-160-160-160zM512 608c-52.992 0-96-43.008-96-96 0-53.024 43.008-96 96-96s96 42.976 96 96c0 52.992-42.944 96-96 96z'
      fill='#444444'
    ></path>
  </SvgIcon>
);

IconPageview1.displayName = 'icon-pageview1';

export default IconPageview1;
