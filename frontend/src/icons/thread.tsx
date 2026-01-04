import type { SVGProps } from 'react';

export function ThreadIcon(props: SVGProps<SVGSVGElement>) {
  return (
    <svg
      xmlns='http://www.w3.org/2000/svg'
      width='24'
      height='24'
      viewBox='0 0 24 24'
      fill='none'
      stroke='currentColor'
      strokeWidth='2'
      strokeLinecap='round'
      strokeLinejoin='round'
      {...props}
    >
      {/* Chat bubbles representing conversation thread */}
      <path d='M7.9 20A9 9 0 1 0 4 16.1L2 22Z' />
      <path d='M15.8 11.8A5 5 0 1 0 13 15l-2 3Z' />

      {/* Connecting dots */}
      <circle cx='12' cy='12' r='1' fill='currentColor' />
      <circle cx='9' cy='9' r='1' fill='currentColor' />
      <circle cx='15' cy='9' r='1' fill='currentColor' />
    </svg>
  );
}
