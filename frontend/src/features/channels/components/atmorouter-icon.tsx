import React from 'react';

interface AtmoRouterIconProps {
  size?: number | string;
  className?: string;
  style?: React.CSSProperties;
}

// AtmoRouter favicon mark: a dark rounded square with a white outlined bolt.
export const AtmoRouterIcon: React.FC<AtmoRouterIconProps> = ({ size = 20, className = '', style = {}, ...rest }) => {
  return (
    <svg
      height={size}
      style={{ flex: '0 0 auto', lineHeight: 1, ...style }}
      viewBox='0 0 32 32'
      width={size}
      xmlns='http://www.w3.org/2000/svg'
      className={className}
      {...rest}
    >
      <title>AtmoRouter</title>
      <rect width='32' height='32' rx='7' fill='#09090b' />
      <path
        d='M18 5L7.5 18.2h7.2l-1.1 8.8L24 13.8h-7.2z'
        fill='none'
        stroke='#fff'
        strokeWidth='2.4'
        strokeLinecap='round'
        strokeLinejoin='round'
      />
    </svg>
  );
};

export default AtmoRouterIcon;
