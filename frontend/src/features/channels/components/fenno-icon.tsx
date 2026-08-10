import React from 'react';

import fennoIconURL from '@/assets/fenno-icon.webp';

interface FennoIconProps extends Omit<React.ImgHTMLAttributes<HTMLImageElement>, 'alt' | 'height' | 'src' | 'width'> {
  size?: number | string;
}

export const FennoIcon: React.FC<FennoIconProps> = ({ size = 20, className = '', style = {}, ...rest }) => {
  return (
    <img
      src={fennoIconURL}
      alt='FennoAI'
      height={size}
      width={size}
      style={{ flex: '0 0 auto', lineHeight: 1, objectFit: 'contain', ...style }}
      className={className}
      {...rest}
    />
  );
};

export default FennoIcon;
