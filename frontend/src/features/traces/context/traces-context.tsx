'use client'

import { createContext, useContext, useState, ReactNode } from 'react'
import { Trace, RequestTrace, Span } from '../data/schema'

interface TracesContextType {
  // Dialog states
  detailDialogOpen: boolean
  setDetailDialogOpen: (open: boolean) => void
  
  // JSON viewer dialog states
  jsonViewerOpen: boolean
  setJsonViewerOpen: (open: boolean) => void
  jsonViewerData: { title: string; data: any } | null
  setJsonViewerData: (data: { title: string; data: any } | null) => void
  
  // Span detail dialog states
  spanDetailOpen: boolean
  setSpanDetailOpen: (open: boolean) => void
  
  // Current selected items
  currentTrace: Trace | null
  setCurrentTrace: (trace: Trace | null) => void
  
  currentRequestTrace: RequestTrace | null
  setCurrentRequestTrace: (requestTrace: RequestTrace | null) => void
  
  currentSpan: Span | null
  setCurrentSpan: (span: Span | null) => void
  
  // Table selection
  selectedTraces: string[]
  setSelectedTraces: (ids: string[]) => void
}

const TracesContext = createContext<TracesContextType | undefined>(undefined)

interface TracesProviderProps {
  children: ReactNode
}

export default function TracesProvider({ children }: TracesProviderProps) {
  const [detailDialogOpen, setDetailDialogOpen] = useState(false)
  const [jsonViewerOpen, setJsonViewerOpen] = useState(false)
  const [jsonViewerData, setJsonViewerData] = useState<{ title: string; data: any } | null>(null)
  const [spanDetailOpen, setSpanDetailOpen] = useState(false)
  const [currentTrace, setCurrentTrace] = useState<Trace | null>(null)
  const [currentRequestTrace, setCurrentRequestTrace] = useState<RequestTrace | null>(null)
  const [currentSpan, setCurrentSpan] = useState<Span | null>(null)
  const [selectedTraces, setSelectedTraces] = useState<string[]>([])

  const value: TracesContextType = {
    detailDialogOpen,
    setDetailDialogOpen,
    jsonViewerOpen,
    setJsonViewerOpen,
    jsonViewerData,
    setJsonViewerData,
    spanDetailOpen,
    setSpanDetailOpen,
    currentTrace,
    setCurrentTrace,
    currentRequestTrace,
    setCurrentRequestTrace,
    currentSpan,
    setCurrentSpan,
    selectedTraces,
    setSelectedTraces,
  }

  return (
    <TracesContext.Provider value={value}>
      {children}
    </TracesContext.Provider>
  )
}

// Also export as named export for convenience
export { TracesProvider }

export function useTracesContext() {
  const context = useContext(TracesContext)
  if (context === undefined) {
    throw new Error('useTracesContext must be used within a TracesProvider')
  }
  return context
}
