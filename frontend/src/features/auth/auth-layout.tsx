interface Props {
  children: React.ReactNode
}

const particles = Array.from({ length: 80 }).map((_, i) => ({
  id: i,
  left: `${Math.random() * 100}%`,
  top: `${Math.random() * 100}%`,
  size: `${Math.random() * 2 + 1}px`,
  delay: `${Math.random() * 5}s`,
  duration: `${Math.random() * 5 + 5}s`,
  color: Math.random() > 0.2 ? '#00C77E' : '#FF2E4D',
}));

export default function AuthLayout({ children }: Props) {
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
      
      {/* Top Navigation */}
      <nav className='relative z-50 flex items-center justify-between p-6'>
        <div className='flex items-center space-x-3'>
          <div className='relative'>
            <svg
              xmlns='http://www.w3.org/2000/svg'
              viewBox='0 0 24 24'
              fill='none'
              stroke='currentColor'
              strokeWidth='2'
              strokeLinecap='round'
              strokeLinejoin='round'
              className='h-8 w-8 text-[#00C77E] animate-neon-pulse'
            >
              <path d='M15 6v12a3 3 0 1 0 3-3H6a3 3 0 1 0 3 3V6a3 3 0 1 0-3 3h12a3 3 0 1 0-3-3' />
            </svg>
          </div>
          <h1 className='text-2xl font-bold text-[#00C77E] animate-neon-pulse font-mono'>
            AxonHub
          </h1>
        </div>
        
        <div className='flex items-center space-x-4'>
          <select className='bg-transparent border border-[#00C77E]/30 text-[#F0F0F0] px-3 py-1 rounded text-sm hover-glow focus-particles'>
            <option value='zh'>中文</option>
            <option value='en'>English</option>
          </select>
        </div>
      </nav>
      
      {/* Main Content Area */}
      <div className='relative z-10 flex min-h-[calc(100vh-88px)]'>
        {/* Right Graphics Section */}
        <div className='flex-1 flex items-center justify-center p-12 relative'>
          {/* Tech Graphics Container */}
          <div className='relative w-full max-w-lg h-96'>
            {/* 3D Data Flow Network */}
            <div className='absolute inset-0'>
              {/* Central AI Chip */}
              <div className='absolute top-1/2 left-1/2 transform -translate-x-1/2 -translate-y-1/2 w-24 h-24 border-2 border-[#00C77E] rounded-lg bg-[#00C77E]/10 animate-neon-pulse'>
                <div className='w-full h-full flex items-center justify-center'>
                  <div className='w-8 h-8 bg-[#00C77E] rounded animate-breathing-glow'></div>
                </div>
              </div>
              
              {/* Data Flow Nodes */}
              <div className='absolute top-8 left-8 w-16 h-16 border border-[#00C77E]/50 rounded-full bg-[#00C77E]/5 animate-particle-float'></div>
              <div className='absolute top-8 right-8 w-12 h-12 border border-[#FF2E4D]/50 rounded-full bg-[#FF2E4D]/5 animate-particle-float animation-delay-1000'></div>
              <div className='absolute bottom-8 left-8 w-14 h-14 border border-[#00C77E]/50 rounded-full bg-[#00C77E]/5 animate-particle-float animation-delay-2000'></div>
              <div className='absolute bottom-8 right-8 w-10 h-10 border border-[#FF2E4D]/50 rounded-full bg-[#FF2E4D]/5 animate-particle-float animation-delay-3000'></div>
              {/* Radar Chart Overlay */}
              <div className='absolute top-4 right-4 w-32 h-32 border border-[#00C77E]/30 rounded-full animate-grid-pulse'>
                <div className='absolute inset-4 border border-[#00C77E]/20 rounded-full'></div>
                <div className='absolute inset-8 border border-[#00C77E]/10 rounded-full'></div>
                <div className='absolute top-1/2 left-0 w-full h-px bg-[#00C77E]/20'></div>
                <div className='absolute top-0 left-1/2 w-px h-full bg-[#00C77E]/20'></div>
              </div>
            </div>
          </div>
        </div>
        
        <div className='absolute inset-0 flex items-center justify-center z-20'>
          <div className='w-full max-w-md animate-fade-in-up animation-delay-300'>
            {children}
          </div>
        </div>
      </div>
      
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
      
      {/* Footer */}
      <footer className='absolute bottom-0 left-0 right-0 z-30 p-6'>
        <div className='flex justify-between items-center text-[#B0B0B0] text-sm'>
          <div className='flex space-x-6'>
            <span> 2024 AxonHub. All rights reserved.</span>
            <a href='#' className='hover:text-[#00C77E] transition-colors'> </a>
            <a href='#' className='hover:text-[#00C77E] transition-colors'> </a>
            <span>© 2024 AxonHub. All rights reserved.</span>
            <a href='#' className='hover:text-[#00C77E] transition-colors'>服务条款</a>
            <a href='#' className='hover:text-[#00C77E] transition-colors'>隐私政策</a>
          </div>
          
          {/* Binary Code Stream */}
          <div className='relative overflow-hidden w-64 h-4'>
            <div className='absolute inset-0 text-[#00C77E]/30 text-xs font-mono animate-binary-stream whitespace-nowrap'>
              01001000 01100101 01101100 01101100 01101111
            </div>
          </div>
        </div>
      </footer>
    </div>
  )
}
