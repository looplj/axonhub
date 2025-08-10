import { useState } from 'react'
import { Copy, Check } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useTranslation } from 'react-i18next'

interface JsonViewerDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  jsonData: any
}

export function JsonViewerDialog({
  open,
  onOpenChange,
  title,
  jsonData,
}: JsonViewerDialogProps) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)

  const formatJson = (data: any) => {
    try {
      if (typeof data === 'string') {
        return JSON.stringify(JSON.parse(data), null, 2)
      }
      return JSON.stringify(data, null, 2)
    } catch {
      return typeof data === 'string' ? data : JSON.stringify(data)
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const formattedJson = formatJson(jsonData)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] max-w-[90vw] w-[90vw]'>
        <DialogHeader className='pb-4'>
          <DialogTitle className='flex items-center start'>
            {title}
            <Button
              className={"ml-4"}
              variant='outline'
              size='sm'
              onClick={() => copyToClipboard(formattedJson)}
            >
              {copied ? (
                <Check className='mr-2 h-4 w-4' />
              ) : (
                <Copy className='mr-2 h-4 w-4' />
              )}
              {copied ? t('requests.dialog.jsonViewer.copied') : t('requests.dialog.jsonViewer.copy')}
            </Button>
          </DialogTitle>
        </DialogHeader>
        <ScrollArea className='h-[72vh] w-full rounded-xs border p-6'>
          <pre className='text-xs whitespace-pre-wrap font-mono'>
            {formattedJson}
          </pre>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}