import { Copy } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'

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
              <Copy className='mr-2 h-4 w-4' />
              复制
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