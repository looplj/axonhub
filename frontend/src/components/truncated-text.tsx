import { useRef, useState, useEffect } from 'react';
import { cn } from '@/lib/utils';

interface TruncatedTextProps {
  children: string;
  className?: string;
}

/**
 * 智能截断文本组件
 * 仅当文本被截断时才显示 title 悬浮提示
 * 使用 ResizeObserver 动态检测截断状态，确保在容器大小变化时也能正确响应
 */
export function TruncatedText({ children, className }: TruncatedTextProps) {
  const textRef = useRef<HTMLSpanElement>(null);
  const [isTruncated, setIsTruncated] = useState(false);

  useEffect(() => {
    const element = textRef.current;
    if (!element) return;

    // 检测文本是否被截断
    const checkTruncation = () => {
      setIsTruncated(element.scrollWidth > element.clientWidth);
    };

    // 初始检测
    checkTruncation();

    // 监听容器尺寸变化
    const resizeObserver = new ResizeObserver(checkTruncation);
    resizeObserver.observe(element);

    return () => resizeObserver.disconnect();
  }, [children]);

  return (
    <span ref={textRef} className={cn('truncate', className)} title={isTruncated ? children : undefined}>
      {children}
    </span>
  );
}
