import React from 'react';

interface OrcaRouterIconProps {
  size?: number | string;
  className?: string;
  style?: React.CSSProperties;
}

export const OrcaRouterIcon: React.FC<OrcaRouterIconProps> = ({ size = 20, className = '', style = {}, ...rest }) => {
  return (
    <svg
      fillRule='evenodd'
      height={size}
      style={{ flex: '0 0 auto', lineHeight: 1, ...style }}
      viewBox='0 0 24 24'
      width={size}
      xmlns='http://www.w3.org/2000/svg'
      className={className}
      {...rest}
    >
      <title>OrcaRouter</title>
      {/* Orca whale silhouette with a router/switching hint */}
      <path
        fill='#0160E6'
        d='M12 2.2c-3.2 0-5.9 1.5-7.7 3.9C2.5 8.5 2 11.2 2 12s.5 3.5 2.3 5.9C6.1 20.3 8.8 21.8 12 21.8s5.9-1.5 7.7-3.9C21.5 15.5 22 12.8 22 12s-.5-3.5-2.3-5.9C17.9 3.7 15.2 2.2 12 2.2Z'
      />
      <path fill='#FFFFFF' d='M7 9.2 14.4 7l-1.2 3H16l-7.4 2.2L9.8 9.2H7Zm7.2 6.2-2-4.8 1.5.6 1.1 2.6 2.4-.7-.5 1.3-2.5.9Z' />
    </svg>
  );
};

export default OrcaRouterIcon;
