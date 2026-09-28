import type React from 'react';
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
        {/* Content */}
        <div className='relative z-10 flex flex-col justify-center px-12 py-16 text-white'>
          <div className='w-full max-w-lg'>
            <div className='mb-8'>
              <h1 className='mb-4 text-4xl font-light text-slate-100'>{t('auth.brand.title')}</h1>
              <h2 className='mb-6 bg-gradient-to-r from-[#F3EEE3] via-[#E6D3AE] to-[#D4B98A] bg-clip-text text-5xl font-semibold tracking-tight text-transparent'>
                AxonHub
              </h2>
              <div className='mb-6 h-px w-16 bg-[#D4B98A]/60' />
              <p className='text-lg leading-relaxed text-slate-300'>{t('auth.brand.description')}</p>
            </div>

            <div className='mt-4'>
              <AutoRouterDiagram />
            </div>
          </div>
        </div>
      </div>

      {/* Right Side - Card/Form */}
      <div className='relative flex min-h-screen w-full items-center justify-center bg-gradient-to-br from-[#F7F4EE] to-[#EDE7DC] lg:w-1/2'>
        {/* Subtle background texture */}
        <div className='absolute inset-0 opacity-30'>
          <div className='absolute inset-0 bg-[radial-gradient(circle_at_50%_50%,rgba(168,132,78,0.08)_0%,transparent_70%)]'></div>
        </div>

        <div id='auth-card-wrapper' data-testid='auth-card-wrapper' className={`relative z-10 w-full ${rightMaxWidthClassName} px-6 py-8 sm:px-8 sm:py-12`}>
          <Card
            className='animate-fade-in-up border-[#E6DFD2] bg-white/90 text-slate-800 shadow-xl shadow-slate-900/10 backdrop-blur-sm transition-all duration-500 hover:shadow-2xl hover:shadow-slate-900/15'
            style={
              {
                // Ensure shadcn variable-based components render with dark-on-light colors inside the white card
                '--foreground': '#1e293b', // slate-800
                '--muted-foreground': '#94a3b8', // slate-400 (for placeholders, help texts)
              } as React.CSSProperties
            }
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
