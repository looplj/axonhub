import { LanguageSwitch } from '@/components/language-switch';

interface Props {
  children: React.ReactNode;
}

export default function AuthLayout({ children }: Props) {
  return (
    <div className='relative min-h-screen overflow-hidden bg-[#1A1A1A]'>
      {/* Static grid background */}
      <div
        className='absolute inset-0 opacity-20'
        style={{
          backgroundImage:
            'linear-gradient(rgba(0,255,157,0.1) 1px, transparent 1px), linear-gradient(90deg, rgba(0,255,157,0.1) 1px, transparent 1px)',
          backgroundSize: '50px 50px',
        }}
      />

      {/* Low-poly network pattern (static) */}
      <div
        className='absolute inset-0'
        style={{
          backgroundImage:
            'radial-gradient(circle at 25% 25%, rgba(0,255,157,0.1) 0%, transparent 50%), ' +
            'radial-gradient(circle at 75% 75%, rgba(255,46,77,0.1) 0%, transparent 50%), ' +
            'radial-gradient(circle at 50% 50%, rgba(240,240,240,0.05) 0%, transparent 50%)',
        }}
      />

      {/* Top Navigation (overlay) */}
      <nav className='absolute top-0 right-0 left-0 z-50 flex items-center justify-between p-6'>
        <div className='flex items-center space-x-3'>
          <img src='/logo.jpg' alt='AxonHub logo' className='h-8 w-8 rounded-sm shadow-sm ring-1 ring-emerald-400/20' />
          <h1 className='bg-gradient-to-r from-emerald-300 to-teal-200 bg-clip-text text-2xl font-semibold text-transparent'>AxonHub</h1>
        </div>

        <div className='flex items-center space-x-2'>
          <LanguageSwitch />
        </div>
      </nav>

      {/* Main Content Area */}
      <main className='relative z-10 min-h-screen'>{children}</main>
    </div>
  );
}
