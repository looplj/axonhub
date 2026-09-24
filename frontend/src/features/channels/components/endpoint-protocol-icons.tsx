import React from 'react';

interface ProtocolIconProps {
  size?: number | string;
  className?: string;
  style?: React.CSSProperties;
}

const baseProps = (size: number | string, className: string, style: React.CSSProperties) => ({
  width: size,
  height: size,
  viewBox: '0 0 24 24',
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.9,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
  style: { flex: '0 0 auto', lineHeight: 1, ...style },
  className,
  xmlns: 'http://www.w3.org/2000/svg',
});

/** Chat completions: a conversation bubble. */
export const ChatProtocolIcon: React.FC<ProtocolIconProps> = ({ size = 16, className = '', style = {}, ...rest }) => (
  <svg aria-hidden='true' {...baseProps(size, className, style)} {...rest}>
    <path d='M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8z' />
    <path d='M8.5 10.5h7M8.5 14h4.5' />
  </svg>
);

/** Responses: a generated response burst, distinct from plain chat. */
export const ResponsesProtocolIcon: React.FC<ProtocolIconProps> = ({ size = 16, className = '', style = {}, ...rest }) => (
  <svg aria-hidden='true' {...baseProps(size, className, style)} {...rest}>
    <path d='M12 3.5c.35 3.1 2.4 5.15 5.5 5.5-3.1.35-5.15 2.4-5.5 5.5-.35-3.1-2.4-5.15-5.5-5.5 3.1-.35 5.15-2.4 5.5-5.5z' />
    <path d='M18 14.5c.17 1.55 1.2 2.58 2.75 2.75-1.55.17-2.58 1.2-2.75 2.75-.17-1.55-1.2-2.58-2.75-2.75 1.55-.17 2.58-1.2 2.75-2.75z' />
  </svg>
);

/** Anthropic messages: the slanted mark of the Messages API. */
export const MessagesProtocolIcon: React.FC<ProtocolIconProps> = ({ size = 16, className = '', style = {}, ...rest }) => (
  <svg aria-hidden='true' {...baseProps(size, className, style)} {...rest}>
    <path d='M6.5 19.5 12 4.5l5.5 15' />
    <path d='M8.4 14.5h7.2' />
  </svg>
);
