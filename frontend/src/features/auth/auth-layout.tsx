import { useTranslation } from 'react-i18next'

interface Props {
  children: React.ReactNode
}

const particles = Array.from({ length: 80 }).map((_, i) => ({
  id: i,
  left: `${Math.random() * 100}%`,
  top: `${Math.random() * 100}%`,
  size: `${Math.random() * 2 + 1}px`,
  delay: `${Math.random() * 5}s`,
  duration: `${Math.random() * 10 + 5}s`,
  color: Math.random() > 0.2 ? '#00C77E' : '#FF2E4D',
}));

export default function AuthLayout({ children }: Props) {
  const { i18n } = useTranslation()

  return (
    <div className='relative min-h-screen overflow-hidden bg-[#1A1A1A] tech'>
      {/* Tech grid background */}
      <div className='absolute inset-0 tech-grid opacity-30'></div>
      
      {/* Low-poly network pattern */}
      <div className='absolute inset-0 low-poly-network'></div>

      {/* Fullscreen Connection Lines */}
      <svg
        className='absolute inset-0 w-full h-full z-0 opacity-40'
        preserveAspectRatio='xMidYMid slice'
        viewBox='0 0 1920 1080'
      >
        <defs>
          <linearGradient id='dataFlow' x1='0%' y1='0%' x2='100%' y2='0%'>
            <stop offset='0%' stopColor='#00C77E' stopOpacity='0' />
            <stop offset='50%' stopColor='#00C77E' stopOpacity='1' />
            <stop offset='100%' stopColor='#00C77E' stopOpacity='0' />
          </linearGradient>
        </defs>
        <line x1='0' y1='0' x2='960' y2='540' stroke='url(#dataFlow)' strokeWidth='2' className='animate-data-flow' />
        <line x1='1920' y1='0' x2='960' y2='540' stroke='url(#dataFlow)' strokeWidth='2' className='animate-data-flow animation-delay-1000' />
        <line x1='0' y1='1080' x2='960' y2='540' stroke='url(#dataFlow)' strokeWidth='2' className='animate-data-flow animation-delay-2000' />
        <line x1='1920' y1='1080' x2='960' y2='540' stroke='url(#dataFlow)' strokeWidth='2' className='animate-data-flow animation-delay-3000' />
      </svg>
      
      {/* Top Navigation (overlay) */}
      <nav className='absolute top-0 left-0 right-0 z-50 flex items-center justify-between p-6'>
        <div className='flex items-center space-x-3'>
          <img
            src='/logo.jpg'
            alt='AxonHub logo'
            className='h-8 w-8 rounded-sm shadow-sm ring-1 ring-emerald-400/20'
          />
          <h1 className='text-2xl font-semibold bg-gradient-to-r from-emerald-300 to-teal-200 bg-clip-text text-transparent'>
            AxonHub
          </h1>
        </div>
        
        <div className='flex items-center space-x-4'>
          <select
            className='bg-transparent border border-emerald-400/30 text-[#F0F0F0] px-3 py-1 rounded text-sm hover-glow focus-particles'
            value={i18n.language}
            onChange={(e) => i18n.changeLanguage(e.target.value)}
          >
            <option value='zh' className='bg-[#1A1A1A] text-[#F0F0F0]'>中文</option>
            <option value='en' className='bg-[#1A1A1A] text-[#F0F0F0]'>English</option>
          </select>
        </div>
      </nav>
      
      {/* Main Content Area - children control layout; full height since header overlays */}
      <main className='relative z-10 min-h-screen'>
        {children}
      </main>
      
      {/* Micro-light Particles Background */}
      <div className='absolute inset-0 overflow-hidden pointer-events-none z-0'>
        {/* Matrix Rain Effect */}
        {/* <div className='absolute top-0 left-1/4 w-px h-full bg-gradient-to-b from-transparent via-[#00C77E]/30 to-transparent animate-matrix-rain'></div>
        <div className='absolute top-0 left-3/4 w-px h-full bg-gradient-to-b from-transparent via-[#00C77E]/20 to-transparent animate-matrix-rain animation-delay-2000'></div>
        <div className='absolute top-0 left-1/2 w-px h-full bg-gradient-to-b from-transparent via-[#FF2E4D]/20 to-transparent animate-matrix-rain animation-delay-4000'></div> */}
        
        {/* Floating Particles */}
        {particles.map((p) => (
          <div
            key={p.id}
            className='absolute rounded-full animate-particle-float'
            style={{
              left: p.left,
              top: p.top,
              width: p.size,
              height: p.size,
              backgroundColor: p.color,
              boxShadow: `0 0 10px ${p.color}`,
              animationDelay: p.delay,
              animationDuration: p.duration,
            }}
          ></div>
        ))}
      </div>
    </div>
  )
}
