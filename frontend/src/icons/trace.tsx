import type { SVGProps } from 'react'

export function TraceIcon(props: SVGProps<SVGSVGElement>) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      {...props}
    >
      {/* Timeline vertical line */}
      <line x1="12" y1="3" x2="12" y2="21" />
      
      {/* Top node */}
      <circle cx="12" cy="5" r="2" />
      
      {/* Middle nodes with branches */}
      <circle cx="12" cy="12" r="2" />
      <line x1="12" y1="12" x2="18" y2="12" />
      <circle cx="18" cy="12" r="1.5" />
      
      {/* Bottom node */}
      <circle cx="12" cy="19" r="2" />
    </svg>
  )
}
