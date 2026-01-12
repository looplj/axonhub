'use client';

import { type ComponentProps, createContext, type HTMLAttributes, useContext, useEffect, useRef, useState } from 'react';
import type { Element } from 'hast';
import { CheckIcon, CopyIcon } from 'lucide-react';
import { type BundledLanguage, codeToHtml, type ShikiTransformer } from 'shiki';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';

type MaskedCodeBlockProps = HTMLAttributes<HTMLDivElement> & {
  displayCode: string;
  realCode: string;
  language: BundledLanguage;
  showLineNumbers?: boolean;
  preRenderedHtml?: {
    light: string;
    dark: string;
  };
};

type MaskedCodeBlockContextType = {
  displayCode: string;
  realCode: string;
};

const MaskedCodeBlockContext = createContext<MaskedCodeBlockContextType>({
  displayCode: '',
  realCode: '',
});

const lineNumberTransformer: ShikiTransformer = {
  name: 'line-numbers',
  line(node: Element, line: number) {
    node.children.unshift({
      type: 'element',
      tagName: 'span',
      properties: {
        className: ['inline-block', 'min-w-10', 'mr-4', 'text-right', 'select-none', 'text-muted-foreground'],
      },
      children: [{ type: 'text', value: String(line) }],
    });
  },
};

export async function highlightMaskedCode(code: string, language: BundledLanguage, showLineNumbers = false) {
  const transformers: ShikiTransformer[] = showLineNumbers ? [lineNumberTransformer] : [];

  return await Promise.all([
    codeToHtml(code, {
      lang: language,
      theme: 'one-light',
      transformers,
    }),
    codeToHtml(code, {
      lang: language,
      theme: 'one-dark-pro',
      transformers,
    }),
  ]);
}

export const MaskedCodeBlock = ({ displayCode, realCode, language, showLineNumbers = false, preRenderedHtml, className, children, ...props }: MaskedCodeBlockProps) => {
  const [html, setHtml] = useState<string>(preRenderedHtml?.light || '');
  const [darkHtml, setDarkHtml] = useState<string>(preRenderedHtml?.dark || '');
  const [isLoading, setIsLoading] = useState(!preRenderedHtml);
  const mounted = useRef(false);

  useEffect(() => {
    if (preRenderedHtml) {
      setHtml(preRenderedHtml.light);
      setDarkHtml(preRenderedHtml.dark);
      setIsLoading(false);
      return;
    }

    setIsLoading(true);
    highlightMaskedCode(displayCode, language, showLineNumbers).then(([light, dark]) => {
      if (!mounted.current) {
        setHtml(light);
        setDarkHtml(dark);
        setIsLoading(false);
        mounted.current = true;
      }
    });

    return () => {
      mounted.current = false;
    };
  }, [displayCode, language, showLineNumbers, preRenderedHtml]);

  return (
    <MaskedCodeBlockContext.Provider value={{ displayCode, realCode }}>
      <div className={cn('group bg-background text-foreground relative w-full overflow-hidden rounded-md border', className)} {...props}>
        <div className='relative'>
          {isLoading ? (
            <div className='flex items-center justify-center p-4'>
              <div className='h-4 w-4 animate-spin rounded-full border-2 border-primary border-t-transparent' />
            </div>
          ) : (
            <>
              <div
                className='[&>pre]:bg-background! [&>pre]:text-foreground! overflow-hidden dark:hidden [&_code]:font-mono [&_code]:text-sm [&>pre]:m-0 [&>pre]:p-4 [&>pre]:text-sm'
                dangerouslySetInnerHTML={{ __html: html }}
              />
              <div
                className='[&>pre]:bg-background! [&>pre]:text-foreground! hidden overflow-hidden dark:block [&_code]:font-mono [&_code]:text-sm [&>pre]:m-0 [&>pre]:p-4 [&>pre]:text-sm'
                dangerouslySetInnerHTML={{ __html: darkHtml }}
              />
            </>
          )}
          {children && <div className='absolute top-2 right-2 flex items-center gap-2'>{children}</div>}
        </div>
      </div>
    </MaskedCodeBlockContext.Provider>
  );
};

export type MaskedCodeBlockCopyButtonProps = ComponentProps<typeof Button> & {
  onCopy?: () => void;
  onError?: (error: Error) => void;
  timeout?: number;
};

export const MaskedCodeBlockCopyButton = ({ onCopy, onError, timeout = 2000, children, className, ...props }: MaskedCodeBlockCopyButtonProps) => {
  const [isCopied, setIsCopied] = useState(false);
  const { realCode } = useContext(MaskedCodeBlockContext);

  const copyToClipboard = async () => {
    if (typeof window === 'undefined' || !navigator?.clipboard?.writeText) {
      onError?.(new Error('Clipboard API not available'));
      return;
    }

    try {
      await navigator.clipboard.writeText(realCode);
      setIsCopied(true);
      onCopy?.();
      setTimeout(() => setIsCopied(false), timeout);
    } catch (error) {
      onError?.(error as Error);
    }
  };

  const Icon = isCopied ? CheckIcon : CopyIcon;

  return (
    <Button className={cn('shrink-0', className)} onClick={copyToClipboard} size='icon' variant='ghost' {...props}>
      {children ?? <Icon size={14} />}
    </Button>
  );
};
