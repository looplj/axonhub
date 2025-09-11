import React from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import AutoRouterDiagram from '../sign-in/components/auto-router-diagram'

export interface TwoColumnAuthProps {
  title: React.ReactNode
  description?: React.ReactNode
  children: React.ReactNode
  rightFooter?: React.ReactNode
  rightMaxWidthClassName?: string // e.g. 'max-w-md'
}

/**
 * TwoColumnAuth
 * Reusable left/right layout used by Sign-In and Initialization pages.
 * Left panel: shared AxonHub brand section and diagram.
 * Right panel: gradient background with a Card shell for page-specific forms.
 */
export default function TwoColumnAuth({
  title,
  description,
  children,
  rightFooter,
  rightMaxWidthClassName = 'max-w-md',
}: TwoColumnAuthProps) {
  return (
    <div className="min-h-screen flex">
      {/* Left Side - Brand/Welcome Section */}
      <div className="hidden lg:flex lg:w-1/2 relative overflow-hidden bg-gradient-to-br from-slate-900/60 via-slate-800/40 to-slate-900/60 backdrop-blur-[1.5px]">
        {/* Elegant background pattern */}
        <div className="absolute inset-0 opacity-10">
          <div className="absolute top-0 left-0 w-full h-full bg-[radial-gradient(circle_at_25%_25%,rgba(255,255,255,0.1)_0%,transparent_50%)]"></div>
          <div className="absolute bottom-0 right-0 w-full h-full bg-[radial-gradient(circle_at_75%_75%,rgba(255,255,255,0.05)_0%,transparent_50%)]"></div>
        </div>

        {/* Content */}
        <div className="relative z-10 flex flex-col justify-center px-12 py-16 text-white">
          <div className="w-full max-w-lg">
            <div className="mb-8">
              <h1 className="text-4xl font-light mb-4 text-slate-100">Unified AI Gateway</h1>
              <h2 className="text-5xl font-bold mb-6 bg-gradient-to-r from-emerald-300 to-teal-200 bg-clip-text text-transparent">AxonHub</h2>
              <p className="text-lg text-slate-300 leading-relaxed">
                Unified OpenAI/Anthropic compatible API with a flexible transformer pipeline, intelligent routing, and comprehensive tracing—built for enterprise reliability.
              </p>
            </div>

            <div className="mt-4">
              <AutoRouterDiagram />
            </div>
          </div>
        </div>

        {/* Decorative elements */}
        <div className="absolute bottom-0 left-0 w-32 h-32 bg-gradient-to-tr from-slate-700/10 to-transparent rounded-full -translate-x-16 translate-y-16"></div>
        <div className="absolute top-0 right-0 w-48 h-48 bg-gradient-to-bl from-slate-600/5 to-transparent rounded-full translate-x-24 -translate-y-24"></div>
      </div>

      {/* Right Side - Card/Form */}
      <div className="w-full lg:w-1/2 flex items-center justify-center bg-gradient-to-br from-slate-50 to-slate-100 relative min-h-screen">
        {/* Subtle background texture */}
        <div className="absolute inset-0 opacity-30">
          <div className="absolute inset-0 bg-[radial-gradient(circle_at_50%_50%,rgba(148,163,184,0.1)_0%,transparent_70%)]"></div>
        </div>

        <div className={`relative z-10 w-full ${rightMaxWidthClassName} px-6 sm:px-8 py-8 sm:py-12`}>
          <Card
            className='backdrop-blur-sm bg-white/90 border-slate-200/60 shadow-xl shadow-slate-900/10 animate-fade-in-up hover:shadow-2xl hover:shadow-slate-900/15 transition-all duration-500 text-slate-800'
            style={{
              // Ensure shadcn variable-based components render with dark-on-light colors inside the white card
              ['--foreground' as any]: '#1e293b', // slate-800
              ['--muted-foreground' as any]: '#94a3b8', // slate-400 (for placeholders, help texts)
            }}
          >
            <CardHeader className='text-center pb-6 sm:pb-8 px-6 sm:px-8 pt-8'>
              <CardTitle className='text-2xl sm:text-3xl font-light text-slate-800 mb-3'>
                {title}
              </CardTitle>
              {description ? (
                <CardDescription className='text-slate-600 text-sm sm:text-base leading-relaxed'>
                  {description}
                </CardDescription>
              ) : null}
            </CardHeader>
            <CardContent className="px-6 sm:px-8 pb-8">
              {children}
            </CardContent>
          </Card>

          {rightFooter ? (
            <div className="mt-6 sm:mt-8 text-center px-4">
              {rightFooter}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  )
}
