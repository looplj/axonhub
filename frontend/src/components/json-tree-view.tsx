'use client';

import * as React from 'react';
import { ChevronRight, ChevronDown, Copy, Check, MoreHorizontal, ChevronUp } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';

/*
MIT License

Copyright (c) 2024 monto

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

/** Context that broadcasts global string-expansion state down the tree. */
const JsonExpandContext = React.createContext<{
  globalStringExpanded: boolean;
  hideArrayIndices: boolean;
  expandDepth: number | 'all';
}>({
  globalStringExpanded: false,
  hideArrayIndices: false,
  expandDepth: 2,
});

type JsonViewerProps = {
  data: any;
  rootName?: string;
  defaultExpanded?: boolean;
  /** Object expansion depth when defaultExpanded=true, or "all" to expand all levels. */
  expandDepth?: number | 'all';
  /** Hide array entry index labels (0,1,2...) for a cleaner JSON view. */
  hideArrayIndices?: boolean;
  /** When true, all string values render their full content. */
  globalStringExpanded?: boolean;
  className?: string;
};

export function JsonViewer({
  data,
  rootName = 'root',
  defaultExpanded = true,
  expandDepth = 2,
  hideArrayIndices = false,
  globalStringExpanded = false,
  className,
}: JsonViewerProps) {
  return (
    <JsonExpandContext.Provider value={{ globalStringExpanded, hideArrayIndices, expandDepth }}>
      <TooltipProvider>
        <div className={cn('w-full min-w-0 overflow-x-hidden [contain:inline-size] font-mono text-sm', className)}>
          <JsonNode name={rootName} data={data} isRoot={true} defaultExpanded={defaultExpanded} />
        </div>
      </TooltipProvider>
    </JsonExpandContext.Provider>
  );
}

type JsonNodeProps = {
  name: string;
  data: any;
  isRoot?: boolean;
  isArrayItem?: boolean;
  defaultExpanded?: boolean;
  level?: number;
};

function JsonNode({ name, data, isRoot = false, isArrayItem = false, defaultExpanded = true, level = 0 }: JsonNodeProps) {
  const { hideArrayIndices, expandDepth } = React.useContext(JsonExpandContext);
  const [isExpanded, setIsExpanded] = React.useState(defaultExpanded);
  const [isCopied, setIsCopied] = React.useState(false);

  const handleToggle = () => setIsExpanded((v) => !v);

  const copyToClipboard = (e: React.MouseEvent) => {
    e.stopPropagation();
    navigator.clipboard.writeText(JSON.stringify(data, null, 2));
    setIsCopied(true);
    setTimeout(() => setIsCopied(false), 2000);
  };

  const dataType = data === null ? 'null' : Array.isArray(data) ? 'array' : typeof data;
  const isExpandable =
    data !== null &&
    data !== undefined &&
    !(data instanceof Date) &&
    (dataType === 'object' || dataType === 'array');
  const itemCount =
    isExpandable && data !== null && data !== undefined ? Object.keys(data).length : 0;

  const childDefaultExpanded =
    !defaultExpanded ? false : expandDepth === 'all' ? true : dataType === 'array' ? true : level < expandDepth;
  const hideArrayItemName = isArrayItem && hideArrayIndices;
  const showName = !hideArrayItemName && name !== '';

  return (
    <div className={cn('group/object box-border min-w-0 overflow-hidden', level > 0 && 'border-border border-l pl-2')}>
      {/* Row: use items-start so the key stays top-aligned when a value expands */}
      <div
        className={cn(
          'hover:bg-muted/50 group/property relative flex w-full min-w-0 max-w-full cursor-pointer items-start gap-1 rounded px-0.5 py-0.5 pr-6',
          isRoot && 'text-primary font-semibold'
        )}
        onClick={isExpandable ? handleToggle : undefined}
      >
        {/* Expand chevron — fixed width, top-aligned */}
        {(isExpandable || !hideArrayItemName) && (
          <div className='mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center'>
            {isExpandable ? (
              isExpanded ? (
                <ChevronDown className='text-muted-foreground h-3 w-3' />
              ) : (
                <ChevronRight className='text-muted-foreground h-3 w-3' />
              )
            ) : null}
          </div>
        )}

        {/* Key label */}
        {showName && <span className='text-primary mt-px shrink-0'>{name}</span>}

        {/* Colon / bracket */}
        {isExpandable ? (
          <span className='text-muted-foreground mt-px shrink-0'>
            {dataType === 'array' ? '[' : '{'}
            {!isExpanded && (
              <span className='text-muted-foreground'>
                {' '}
                {itemCount} {itemCount === 1 ? 'item' : 'items'} {dataType === 'array' ? ']' : '}'}
              </span>
            )}
          </span>
        ) : (
          showName && <span className='text-muted-foreground mt-px shrink-0'>:</span>
        )}

        {/* Leaf value — takes remaining width, allows text to truncate */}
        {!isExpandable && (
          <div className='min-w-0 flex-1 overflow-hidden'>
            <JsonValue data={data} />
          </div>
        )}

        {/* Copy button */}
        <button
          onClick={copyToClipboard}
          className='hover:bg-muted absolute top-0.5 right-0.5 shrink-0 rounded p-1 opacity-0 transition-opacity group-hover/property:opacity-100'
          title='Copy to clipboard'
        >
          {isCopied ? (
            <Check className='h-3 w-3 text-green-500' />
          ) : (
            <Copy className='text-muted-foreground h-3 w-3' />
          )}
        </button>
      </div>

      {isExpandable && isExpanded && data !== null && data !== undefined && (
        <div className='pl-2'>
          {Object.keys(data).map((key) => (
            <JsonNode
              key={key}
              name={key}
              data={data[key]}
              isArrayItem={dataType === 'array'}
              level={level + 1}
              defaultExpanded={childDefaultExpanded}
            />
          ))}
          <div className='text-muted-foreground py-0.5 pl-2 text-xs'>
            {dataType === 'array' ? ']' : '}'}
          </div>
        </div>
      )}
    </div>
  );
}

/** Unescape doubly-escaped JSON sequences (literal \n → newline, etc.). */
function unescapeJsonString(str: string): string {
  return str
    .replace(/\\n/g, '\n')
    .replace(/\\t/g, '\t')
    .replace(/\\r/g, '\r')
    .replace(/\\"/g, '"')
    .replace(/\\\\/g, '\\');
}

/** Try to parse a string as JSON. Returns parsed value or null. */
function tryParseJson(str: string): any {
  const trimmed = str.trim();
  if (
    (trimmed.startsWith('{') && trimmed.endsWith('}')) ||
    (trimmed.startsWith('[') && trimmed.endsWith(']'))
  ) {
    try {
      return JSON.parse(trimmed);
    } catch {
      /* not valid JSON */
    }
  }
  return null;
}

/**
 * Normalize multiline text for UI display so heavy leading indentation
 * does not push content off-screen in narrow containers.
 */
function normalizeMultilineForDisplay(str: string): string {
  const lines = str.replace(/\t/g, '  ').split('\n');
  if (lines.length <= 1) return str;

  // Ignore first line when measuring common indent because many values start
  // at column 0 then have heavily-indented continuation lines.
  const continuationLines = lines.slice(1);
  const indents = continuationLines
    .filter((line) => line.trim().length > 0)
    .map((line) => {
      const match = line.match(/^\s+/u);
      return match ? match[0].length : 0;
    })
    .filter((indent) => indent > 0);

  if (indents.length === 0) return str;

  const minIndent = Math.min(...indents);
  return lines
    .map((line, index) => {
      if (index === 0) return line;
      if (line.trim().length === 0) return line;

      const dedented = line.replace(new RegExp(`^\\s{0,${minIndent}}`, 'u'), '');
      const extraIndent = dedented.match(/^\s+/u)?.[0]?.length ?? 0;

      // Keep only a tiny visual indent per line to avoid right-shift overflow.
      if (extraIndent > 2) {
        return `  ${dedented.trimStart()}`;
      }
      return dedented;
    })
    .join('\n');
}

function JsonValue({ data }: { data: any }) {
  const { globalStringExpanded } = React.useContext(JsonExpandContext);
  const [localExpanded, setLocalExpanded] = React.useState<boolean | null>(null);
  const [showParsed, setShowParsed] = React.useState(false);
  const dataType = typeof data;

  // Derived: local override takes precedence, otherwise follow global.
  const isExpanded = localExpanded ?? globalStringExpanded;

  // Reset local override whenever the global toggle changes.
  React.useEffect(() => {
    setLocalExpanded(null);
    setShowParsed(false);
  }, [globalStringExpanded]);

  if (data === null) return <span className='text-rose-500'>null</span>;
  if (data === undefined) return <span className='text-muted-foreground'>undefined</span>;
  if (data instanceof Date) return <span className='text-purple-500'>{data.toISOString()}</span>;

  switch (dataType) {
    case 'string': {
      const unescaped = unescapeJsonString(data);
      const normalized = normalizeMultilineForDisplay(unescaped);
      const parsedJson = tryParseJson(data);

      if (parsedJson && showParsed) {
        return (
          <div className='flex w-full min-w-0 flex-col gap-1'>
            <div className='flex items-center gap-2'>
              <span className='text-muted-foreground text-[10px] italic'>JSON</span>
              <button
                className='text-muted-foreground hover:text-foreground text-[10px] underline'
                onClick={(e) => {
                  e.stopPropagation();
                  setShowParsed(false);
                }}
              >
                raw
              </button>
            </div>
            <div className='border-border w-full min-w-0 rounded border p-2'>
              <JsonNode name='' data={parsedJson} level={0} defaultExpanded={true} />
            </div>
          </div>
        );
      }

      return (
        <div
          className='group/str flex w-full min-w-0 max-w-full cursor-pointer items-start text-emerald-500'
          onClick={(e) => {
            e.stopPropagation();
            setLocalExpanded(!isExpanded);
          }}
        >
          {isExpanded ? (
            <pre
              className='m-0 block min-w-0 max-w-full flex-1 overflow-hidden font-inherit text-inherit'
              style={{
                whiteSpace: 'pre-wrap',
                overflowWrap: 'anywhere',
                wordBreak: 'break-word',
                tabSize: 2,
              }}
            >{`"${normalized}"`}</pre>
          ) : (
            <Tooltip delayDuration={300}>
              <TooltipTrigger asChild>
                <span
                  className='block min-w-0 max-w-full flex-1'
                  style={{
                    whiteSpace: 'nowrap',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                  }}
                >{`"${normalized}"`}</span>
              </TooltipTrigger>
              <TooltipContent side='bottom' className='max-w-md p-2 text-xs break-words'>
                <span className='whitespace-pre-wrap'>
                  {`"${normalized.substring(0, 300)}${normalized.length > 300 ? '…' : ''}"`}
                </span>
              </TooltipContent>
            </Tooltip>
          )}
          <span className='ml-1 shrink-0 opacity-0 transition-opacity group-hover/str:opacity-100'>
            {isExpanded ? (
              <ChevronUp className='text-muted-foreground h-3 w-3' />
            ) : (
              <MoreHorizontal className='text-muted-foreground h-3 w-3' />
            )}
          </span>
          {parsedJson && isExpanded && (
            <button
              className='text-muted-foreground hover:text-foreground ml-1 shrink-0 rounded px-1 text-[10px] underline'
              onClick={(e) => {
                e.stopPropagation();
                setShowParsed(true);
              }}
            >
              parse
            </button>
          )}
        </div>
      );
    }
    case 'number':
      return <span className='text-amber-500'>{data}</span>;
    case 'boolean':
      return <span className='text-blue-500'>{data.toString()}</span>;
    default:
      return <span>{String(data)}</span>;
  }
}
