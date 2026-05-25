import React from 'react';

interface AtlasCloudIconProps {
  size?: number | string;
  className?: string;
  style?: React.CSSProperties;
}

export const AtlasCloudIcon: React.FC<AtlasCloudIconProps> = ({
  size = 20,
  className = '',
  style = {},
  ...rest
}) => {
  return (
    <svg
      height={size}
      style={{ flex: '0 0 auto', lineHeight: 1, ...style }}
      viewBox="20.73 315.14 167.3 161.72"
      width={size}
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      {...rest}
    >
      <title>AtlasCloud</title>
      <path
        fill="currentColor"
        d="M104.38,315.14L20.73,476.86c38.73-15.32,70.03-17.6,98.12-16.26l-12.85-28.4c-5.63-.57-21.86-.57-29.57,1.45l27.95-62.14s42.06,91.31,42.13,91.32c8.09,1.27,29.8,8.55,41.52,14.03l-83.66-161.72Z"
      />
    </svg>
  );
};

export default AtlasCloudIcon;
