import React from 'react';

import runcrateIconURL from '@/assets/runcrate-icon.webp';

interface RuncrateIconProps extends Omit<React.ImgHTMLAttributes<HTMLImageElement>, 'alt' | 'height' | 'src' | 'width'> {
  size?: number | string;
}

export const RuncrateIcon: React.FC<RuncrateIconProps> = ({ size = 20, className = '', style = {}, ...rest }) => {
  return (
    <img
      src={runcrateIconURL}
      alt='Runcrate'
      height={size}
      width={size}
      style={{ flex: '0 0 auto', lineHeight: 1, objectFit: 'contain', ...style }}
      className={className}
      {...rest}
    />
  );
};

export default RuncrateIcon;
