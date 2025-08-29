'use client'

import * as React from 'react'
import { ChevronRight, ChevronDown, Copy, Check, MoreHorizontal, ChevronUp } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'

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

type JsonViewerProps = {
  data: any
  rootName?: string
  defaultExpanded?: boolean
  className?: string
}

export function JsonViewer({ data, rootName = 'root', defaultExpanded = true, className }: JsonViewerProps) {
  return (
    <TooltipProvider>
      <div className={cn('font-mono text-sm', className)}>
        <JsonNode name={rootName} data={data} isRoot={true} defaultExpanded={defaultExpanded} />
      </div>
    </TooltipProvider>
  )
}

type JsonNodeProps = {
  name: string
  data: any
  isRoot?: boolean
  defaultExpanded?: boolean
  level?: number
}

function JsonNode({ name, data, isRoot = false, defaultExpanded = true, level = 0 }: JsonNodeProps) {
  const [isExpanded, setIsExpanded] = React.useState(defaultExpanded)
  const [isCopied, setIsCopied] = React.useState(false)

  const handleToggle = () => {
    setIsExpanded(!isExpanded)
  }

  const copyToClipboard = (e: React.MouseEvent) => {
    e.stopPropagation()
    navigator.clipboard.writeText(JSON.stringify(data, null, 2))
    setIsCopied(true)
    setTimeout(() => setIsCopied(false), 2000)
  }

  const dataType = data === null ? 'null' : Array.isArray(data) ? 'array' : typeof data
  const isExpandable =
    data !== null && data !== undefined && !(data instanceof Date) && (dataType === 'object' || dataType === 'array')
  const itemCount = isExpandable && data !== null && data !== undefined ? Object.keys(data).length : 0

  return (
    <div className={cn('group/object pl-4', level > 0 && 'border-border border-l')}>
      <div
        className={cn(
          'hover:bg-muted/50 group/property -ml-4 flex cursor-pointer items-center gap-1 rounded px-1 py-1',
          isRoot && 'text-primary font-semibold'
        )}
        onClick={isExpandable ? handleToggle : undefined}
      >
        {isExpandable ? (
          <div className='flex h-4 w-4 items-center justify-center'>
            {isExpanded ? (
              <ChevronDown className='text-muted-foreground h-3.5 w-3.5' />
            ) : (
              <ChevronRight className='text-muted-foreground h-3.5 w-3.5' />
            )}
          </div>
        ) : (
          <div className='w-4' />
        )}

        <span className='text-primary'>{name}</span>

        <span className='text-muted-foreground'>
          {isExpandable ? (
            <>
              {dataType === 'array' ? '[' : '{'}
              {!isExpanded && (
                <span className='text-muted-foreground'>
                  {' '}
                  {itemCount} {itemCount === 1 ? 'item' : 'items'} {dataType === 'array' ? ']' : '}'}
                </span>
              )}
            </>
          ) : (
            ':'
          )}
        </span>

        {!isExpandable && <JsonValue data={data} />}

        {!isExpandable && <div className='w-3.5' />}

        <button
          onClick={copyToClipboard}
          className='hover:bg-muted ml-auto rounded p-1 opacity-0 group-hover/property:opacity-100'
          title='Copy to clipboard'
        >
          {isCopied ? (
            <Check className='h-3.5 w-3.5 text-green-500' />
          ) : (
            <Copy className='text-muted-foreground h-3.5 w-3.5' />
          )}
        </button>
      </div>

      {isExpandable && isExpanded && data !== null && data !== undefined && (
        <div className='pl-4'>
          {Object.keys(data).map((key) => (
            <JsonNode
              key={key}
              name={dataType === 'array' ? `${key}` : key}
              data={data[key]}
              level={level + 1}
              defaultExpanded={level < 1}
            />
          ))}
          <div className='text-muted-foreground py-1 pl-4'>{dataType === 'array' ? ']' : '}'}</div>
        </div>
      )}
    </div>
  )
}

// Update the JsonValue function to make the entire row clickable with an expand icon
function JsonValue({ data }: { data: any }) {
  const [isExpanded, setIsExpanded] = React.useState(false)
  const dataType = typeof data
  const TEXT_LIMIT = 80 // Character limit before truncation

  if (data === null) {
    return <span className='text-rose-500'>null</span>
  }

  if (data === undefined) {
    return <span className='text-muted-foreground'>undefined</span>
  }

  if (data instanceof Date) {
    return <span className='text-purple-500'>{data.toISOString()}</span>
  }

  switch (dataType) {
    case 'string':
      if (data.length > TEXT_LIMIT) {
        return (
          <div
            className='group relative flex flex-1 cursor-pointer items-center text-emerald-500'
            onClick={(e) => {
              e.stopPropagation()
              setIsExpanded(!isExpanded)
            }}
          >
            {`"`}
            {isExpanded ? (
              <span className='inline-block max-w-full'>{data}</span>
            ) : (
              <Tooltip delayDuration={300}>
                <TooltipTrigger asChild>
                  <span className='inline-block max-w-full'>{data.substring(0, TEXT_LIMIT)}...</span>
                </TooltipTrigger>
                <TooltipContent side='bottom' className='max-w-md p-2 text-xs break-words'>
                  {data}
                </TooltipContent>
              </Tooltip>
            )}
            {`"`}
            <div className='absolute top-1/2 right-0 translate-x-[calc(100%+4px)] -translate-y-1/2 opacity-0 transition-opacity group-hover:opacity-100'>
              {isExpanded ? (
                <ChevronUp className='text-muted-foreground h-3 w-3' />
              ) : (
                <MoreHorizontal className='text-muted-foreground h-3 w-3' />
              )}
            </div>
          </div>
        )
      }
      return <span className='text-emerald-500'>{`"${data}"`}</span>
    case 'number':
      return <span className='text-amber-500'>{data}</span>
    case 'boolean':
      return <span className='text-blue-500'>{data.toString()}</span>
    default:
      return <span>{String(data)}</span>
  }
}
