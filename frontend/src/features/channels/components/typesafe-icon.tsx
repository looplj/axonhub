import React from 'react';

interface TypeSafeIconProps {
  size?: number | string;
  className?: string;
  style?: React.CSSProperties;
}

export const TypeSafeIcon: React.FC<TypeSafeIconProps> = ({
  size = 20,
  className = '',
  style = {},
  ...rest
}) => {
  return (
    <svg
      height={size}
      style={{ flex: '0 0 auto', lineHeight: 1, ...style }}
      viewBox='0 0 128 128'
      width={size}
      xmlns='http://www.w3.org/2000/svg'
      className={className}
      {...rest}
    >
      <title>TypeSafe</title>
      <defs>
        <linearGradient id='typesafe-grad' x1='0%' y1='0%' x2='100%' y2='100%'>
          <stop offset='0%' stopColor='#0ea5e9' />
          <stop offset='50%' stopColor='#3b82f6' />
          <stop offset='100%' stopColor='#6366f1' />
        </linearGradient>
      </defs>
      <rect width='128' height='128' rx='28' fill='url(#typesafe-grad)' />
      {/* Dynamic abstract T & Checkmark shape signifying Type Safety and Precision */}
      <path
        d='M36 42C36 39.7909 37.7909 38 40 38H88C90.2091 38 92 39.7909 92 42V48C92 50.2091 90.2091 52 88 52H69V96C69 98.2091 67.2091 100 65 100H59C56.7909 100 55 98.2091 55 96V52H40C37.7909 52 36 50.2091 36 48V42Z'
        fill='#FFFFFF'
      />
      <circle cx='86' cy='78' r='10' fill='#22c55e' />
      <path
        d='M81.5 78L84.5 81L90.5 75'
        stroke='#FFFFFF'
        strokeWidth='2.5'
        strokeLinecap='round'
        strokeLinejoin='round'
      />
    </svg>
  );
};

export default TypeSafeIcon;
