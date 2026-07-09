import React from 'react';

interface RequestyIconProps {
  size?: number | string;
  className?: string;
  style?: React.CSSProperties;
}

export const RequestyIcon: React.FC<RequestyIconProps> = ({
  size = 20,
  className = '',
  style = {},
  ...rest
}) => {
  return (
    <svg
      height={size}
      style={{ flex: '0 0 auto', lineHeight: 1, ...style }}
      viewBox="0 0 24 24"
      width={size}
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      {...rest}
    >
      <title>Requesty</title>
      <rect width="24" height="24" rx="5" fill="#0B0B0F" />
      <path
        d="M7 6h6.2c2.65 0 4.3 1.5 4.3 3.85 0 1.75-.95 3.05-2.55 3.55L18 18h-2.75l-2.7-4.25H9.5V18H7V6Zm2.5 2.1v3.6h3.4c1.3 0 2.05-.7 2.05-1.8 0-1.1-.75-1.8-2.05-1.8H9.5Z"
        fill="#FFFFFF"
      />
    </svg>
  );
};

export default RequestyIcon;
