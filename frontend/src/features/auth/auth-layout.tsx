import { AxonHubLogo } from '@/components/axonhub-logo';
import { LanguageSwitch } from '@/components/language-switch';
import './sign-in/login-styles.css';

interface Props {
  children: React.ReactNode;
}

export default function AuthLayout({ children }: Props) {
  return (
    <div className='tech relative min-h-screen overflow-hidden bg-[#111418]'>
      {/* Tech grid background */}
      <div className='tech-grid absolute inset-0 opacity-30'></div>

      {/* Low-poly network pattern */}
      <div className='low-poly-network absolute inset-0'></div>

      {/* Fullscreen Connection Lines */}
      <svg className='absolute inset-0 z-0 h-full w-full opacity-40' preserveAspectRatio='xMidYMid slice' viewBox='0 0 1920 1080'>
        <defs>
          <linearGradient id='dataFlow' x1='0%' y1='0%' x2='100%' y2='0%'>
            <stop offset='0%' stopColor='#D4B98A' stopOpacity='0' />
            <stop offset='50%' stopColor='#D4B98A' stopOpacity='1' />
            <stop offset='100%' stopColor='#D4B98A' stopOpacity='0' />
          </linearGradient>
        </defs>
        <line x1='0' y1='0' x2='960' y2='540' stroke='url(#dataFlow)' strokeWidth='2' className='animate-data-flow' />
        <line
          x1='1920'
          y1='0'
          x2='960'
          y2='540'
          stroke='url(#dataFlow)'
          strokeWidth='2'
          className='animate-data-flow animation-delay-1000'
        />
        <line
          x1='0'
          y1='1080'
          x2='960'
          y2='540'
          stroke='url(#dataFlow)'
          strokeWidth='2'
          className='animate-data-flow animation-delay-2000'
        />
        <line
          x1='1920'
          y1='1080'
          x2='960'
          y2='540'
          stroke='url(#dataFlow)'
          strokeWidth='2'
          className='animate-data-flow animation-delay-3000'
        />
      </svg>

      {/* Top Navigation (overlay) */}
      <nav className='absolute top-0 right-0 left-0 z-50 flex items-center justify-between p-6'>
        <div className='flex items-center space-x-3'>
          <AxonHubLogo className='size-8 text-[#F3EEE3] [--logo-accent:#D4B98A]' />
          <h1 className='text-2xl font-semibold tracking-tight text-[#F3EEE3]'>AxonHub</h1>
        </div>

        <div className='flex items-center space-x-2'>
          <LanguageSwitch />
        </div>
      </nav>

      {/* Main Content Area - children control layout; full height since header overlays */}
      <main className='relative z-10 min-h-screen'>{children}</main>
    </div>
  );
}
