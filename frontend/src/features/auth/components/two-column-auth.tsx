import React from 'react';
import { useTranslation } from 'react-i18next';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import AutoRouterDiagram from '../sign-in/components/auto-router-diagram';

export interface TwoColumnAuthProps {
  title: React.ReactNode;
  description?: React.ReactNode;
  children: React.ReactNode;
  rightFooter?: React.ReactNode;
  rightMaxWidthClassName?: string; // e.g. 'max-w-md'
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
  const { t } = useTranslation();
  return (
    <div className='flex min-h-screen'>
      {/* Left Side - Brand/Welcome Section */}
      <div className='relative hidden overflow-hidden bg-gradient-to-br from-slate-900/60 via-slate-800/40 to-slate-900/60 backdrop-blur-[1.5px] lg:flex lg:w-1/2'>
        {/* Elegant background pattern */}
        <div className='absolute inset-0 opacity-10'>
          <div className='absolute top-0 left-0 h-full w-full bg-[radial-gradient(circle_at_25%_25%,rgba(255,255,255,0.1)_0%,transparent_50%)]'></div>
          <div className='absolute right-0 bottom-0 h-full w-full bg-[radial-gradient(circle_at_75%_75%,rgba(255,255,255,0.05)_0%,transparent_50%)]'></div>
        </div>

        {/* Content */}
        <div className='relative z-10 flex flex-col justify-center px-12 py-16 text-white'>
          <div className='w-full max-w-lg'>
            <div className='mb-8'>
              <h1 className='mb-4 text-4xl font-light text-slate-100'>{t('auth.brand.title')}</h1>
              <h2 className='mb-6 bg-gradient-to-r from-emerald-300 to-teal-200 bg-clip-text text-5xl font-bold text-transparent'>
                AxonHub
              </h2>
              <p className='text-lg leading-relaxed text-slate-300'>{t('auth.brand.description')}</p>
            </div>

            <div className='mt-4'>
              <AutoRouterDiagram />
            </div>
          </div>
        </div>

        {/* Decorative elements */}
        <div className='absolute bottom-0 left-0 h-32 w-32 -translate-x-16 translate-y-16 rounded-full bg-gradient-to-tr from-slate-700/10 to-transparent'></div>
        <div className='absolute top-0 right-0 h-48 w-48 translate-x-24 -translate-y-24 rounded-full bg-gradient-to-bl from-slate-600/5 to-transparent'></div>
      </div>

      {/* Right Side - Card/Form */}
      <div className='relative flex min-h-screen w-full items-center justify-center bg-gradient-to-br from-slate-50 to-slate-100 lg:w-1/2'>
        {/* Subtle background texture */}
        <div className='absolute inset-0 opacity-30'>
          <div className='absolute inset-0 bg-[radial-gradient(circle_at_50%_50%,rgba(148,163,184,0.1)_0%,transparent_70%)]'></div>
        </div>

        <div className={`relative z-10 w-full ${rightMaxWidthClassName} px-6 py-8 sm:px-8 sm:py-12`}>
          <Card
            className='animate-fade-in-up border-slate-200/60 bg-white/90 text-slate-800 shadow-xl shadow-slate-900/10 backdrop-blur-sm transition-all duration-500 hover:shadow-2xl hover:shadow-slate-900/15'
            style={{
              // Ensure shadcn variable-based components render with dark-on-light colors inside the white card
              ['--foreground' as any]: '#1e293b', // slate-800
              ['--muted-foreground' as any]: '#94a3b8', // slate-400 (for placeholders, help texts)
            }}
          >
            <CardHeader className='px-6 pt-8 pb-6 text-center sm:px-8 sm:pb-8'>
              <CardTitle className='mb-3 text-2xl font-light text-slate-800 sm:text-3xl'>{title}</CardTitle>
              {description ? (
                <CardDescription className='text-sm leading-relaxed text-slate-600 sm:text-base'>{description}</CardDescription>
              ) : null}
            </CardHeader>
            <CardContent className='px-6 pb-8 sm:px-8'>{children}</CardContent>
          </Card>

          {rightFooter ? <div className='mt-6 px-4 text-center sm:mt-8'>{rightFooter}</div> : null}
        </div>
      </div>
    </div>
  );
}
