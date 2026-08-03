import React from 'react';

interface EvolinkIconProps {
  size?: number | string;
  className?: string;
  style?: React.CSSProperties;
}

export const EvolinkIcon: React.FC<EvolinkIconProps> = ({
  size = 20,
  className = '',
  style = {},
  ...rest
}) => {
  return (
    <svg
      height={size}
      style={{ flex: '0 0 auto', lineHeight: 1, ...style }}
      viewBox="0 0 124 104"
      width={size}
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      {...rest}
    >
      <title>EvoLink</title>
      <g fill="currentColor">
        {/* "E" */}
        <rect x="12" y="14" width="22" height="76" />
        <rect x="12" y="14" width="64" height="20" />
        <rect x="12" y="44" width="50" height="18" />
        <rect x="12" y="70" width="64" height="20" />
        {/* ">" chevron */}
        <polygon points="92,14 120,52 92,90 80,90 104,52 80,14" />
      </g>
    </svg>
  );
};

export default EvolinkIcon;
