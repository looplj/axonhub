import React from 'react';

interface NanoGPTIconProps {
  size?: number | string;
  className?: string;
  style?: React.CSSProperties;
}

export const NanoGPTIcon: React.FC<NanoGPTIconProps> = ({
  size = 20,
  className = '',
  style = {},
  ...rest
}) => {
  return (
    <svg
      fill="currentColor"
      fillRule="evenodd"
      height={size}
      style={{ flex: '0 0 auto', lineHeight: 1, ...style }}
      viewBox="0 0 24 24"
      width={size}
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      {...rest}
    >
      <title>NanoGPT</title>
      <path
        clipRule="evenodd"
        d="M12 1.5L3.25 6.5V17.5L12 22.5L20.75 17.5V6.5L12 1.5ZM12 3.5L18.75 7.5V16.5L12 20.5L5.25 16.5V7.5L12 3.5ZM12 6L8.5 8V16L12 18L15.5 16V8L12 6ZM12 8.5L14 9.5V14.5L12 15.5L10 14.5V9.5L12 8.5Z"
        fill="currentColor"
        fillRule="evenodd"
      />
    </svg>
  );
};

export default NanoGPTIcon;