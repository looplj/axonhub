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
      fillRule="evenodd"
      height={size}
      style={{ flex: '0 0 auto', lineHeight: 1, ...style }}
      viewBox="0 0 181.45 186.88"
      width={size}
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      {...rest}
    >
      <title>NanoGPT</title>
      <defs>
        <linearGradient
          id="nanoGradient"
          x1="43.86"
          y1="128.24"
          x2="145.31"
          y2="128.24"
          gradientUnits="userSpaceOnUse"
        >
          <stop offset="0" stopColor="#015a9e" />
          <stop offset="1" stopColor="#11e9bb" />
        </linearGradient>
        <linearGradient
          id="nanoGradient2"
          x1="44.1"
          y1="49.57"
          x2="146.51"
          y2="49.57"
          xlinkHref="#nanoGradient"
        />
        <linearGradient
          id="nanoGradient3"
          x1="2.6"
          y1="69.86"
          x2="53.68"
          y2="69.86"
          xlinkHref="#nanoGradient"
        />
        <linearGradient
          id="nanoGradient4"
          x1="109.82"
          y1="118.94"
          x2="178.89"
          y2="118.94"
          xlinkHref="#nanoGradient"
        />
        <linearGradient
          id="nanoGradient5"
          x1="129.51"
          y1="46.47"
          x2="173.61"
          y2="46.47"
          xlinkHref="#nanoGradient"
        />
        <linearGradient
          id="nanoGradient6"
          x1="12.44"
          y1="122.84"
          x2="65.92"
          y2="122.84"
          xlinkHref="#nanoGradient"
        />
      </defs>
      <g>
        <polygon
          fill="url(#nanoGradient)"
          points="145.31 72.24 93.39 184.24 43.86 103.66 145.31 72.24"
        />
        <polyline
          fill="url(#nanoGradient2)"
          points="106.21 2.68 146.51 64.44 44.1 96.46 70.86 27.88"
        />
        <polygon
          fill="url(#nanoGradient3)"
          points="30.06 42.79 53.68 52.46 36.31 96.93 2.6 78.59 30.06 42.79"
        />
        <polygon
          fill="url(#nanoGradient4)"
          points="109.82 166.73 153.7 71.14 178.89 81.47 109.82 166.73"
        />
        <polygon
          fill="url(#nanoGradient5)"
          points="173.61 71.76 154.18 63.48 129.51 25.14 134.6 21.19 173.61 71.76"
        />
        <polygon
          fill="url(#nanoGradient6)"
          points="65.92 153.78 12.44 91.91 34.91 104.09 35.83 104.85 65.92 153.78"
        />
      </g>
    </svg>
  );
};

export default NanoGPTIcon;
