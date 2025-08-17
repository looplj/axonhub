interface Props {
  children: React.ReactNode
}

export default function AuthLayout({ children }: Props) {
  return (
    <div className='relative min-h-screen overflow-hidden bg-[#1A1A1A] tech'>
      {/* Tech grid background */}
      <div className='absolute inset-0 tech-grid opacity-30'></div>
      
      {/* Low-poly network pattern */}
      <div className='absolute inset-0 low-poly-network'></div>
      
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
        {/* Left Content Section */}
        <div className='flex-1 flex items-center justify-center p-12'>
          <div className='max-w-2xl space-y-8 animate-fade-in-up'>
            <div className='space-y-4'>
              <h1 className='text-5xl font-bold text-[#F0F0F0] leading-tight'>
                聚合AI之力，
                <span className='text-[#00C77E] animate-neon-pulse'>精准调校</span>
                每一段对话
              </h1>
              <p className='text-xl text-[#B0B0B0] font-light'>
                一站式API管理 × Prompt工程实验室
              </p>
            </div>
            
            {/* Feature Points */}
            <div className='space-y-4'>
              <div className='flex items-center space-x-3 animate-fade-in-up animation-delay-300'>
                <div className='w-2 h-2 bg-[#00C77E] rounded-full animate-neon-pulse'></div>
                <span className='text-[#F0F0F0]'>支持20+主流AI平台API无缝切换</span>
              </div>
              <div className='flex items-center space-x-3 animate-fade-in-up animation-delay-500'>
                <div className='w-2 h-2 bg-[#00C77E] rounded-full animate-neon-pulse'></div>
                <span className='text-[#F0F0F0]'>可视化Prompt性能评估仪表盘</span>
              </div>
              <div className='flex items-center space-x-3 animate-fade-in-up animation-delay-700'>
                <div className='w-2 h-2 bg-[#00C77E] rounded-full animate-neon-pulse'></div>
                <span className='text-[#F0F0F0]'>团队协作式调试工作流</span>
              </div>
            </div>
            
            {/* Action Buttons */}
            <div className='flex space-x-4 animate-fade-in-up animation-delay-1000'>
              <button className='bg-[#00C77E] text-[#1A1A1A] px-8 py-4 rounded-lg font-semibold hover:bg-[#00CC7E] transition-all duration-300 hover-glow animate-neon-pulse'>
                立即开始
              </button>
              <button className='border border-[#00C77E] text-[#00C77E] px-8 py-4 rounded-lg font-semibold hover:bg-[#00C77E]/10 transition-all duration-300 hover-glow'>
                观看演示
              </button>
            </div>
          </div>
        </div>
        
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
              
              {/* Connection Lines */}
              <svg className='absolute inset-0 w-full h-full' viewBox='0 0 400 400'>
                <defs>
                  <linearGradient id='dataFlow' x1='0%' y1='0%' x2='100%' y2='0%'>
                    <stop offset='0%' stopColor='#00C77E' stopOpacity='0' />
                    <stop offset='50%' stopColor='#00C77E' stopOpacity='1' />
                    <stop offset='100%' stopColor='#00C77E' stopOpacity='0' />
                  </linearGradient>
                </defs>
                <line x1='50' y1='50' x2='200' y2='200' stroke='url(#dataFlow)' strokeWidth='2' className='animate-data-flow' />
                <line x1='350' y1='50' x2='200' y2='200' stroke='url(#dataFlow)' strokeWidth='2' className='animate-data-flow animation-delay-1000' />
                <line x1='50' y1='350' x2='200' y2='200' stroke='url(#dataFlow)' strokeWidth='2' className='animate-data-flow animation-delay-2000' />
                <line x1='350' y1='350' x2='200' y2='200' stroke='url(#dataFlow)' strokeWidth='2' className='animate-data-flow animation-delay-3000' />
              </svg>
              
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
        
        {/* Centered Login Form */}
        <div className='absolute inset-0 flex items-center justify-center z-20'>
          <div className='w-full max-w-md animate-fade-in-up animation-delay-300'>
            {children}
          </div>
        </div>
      </div>
      
      {/* Micro-light Particles Background */}
      <div className='absolute inset-0 overflow-hidden pointer-events-none z-0'>
        {/* Matrix Rain Effect */}
        <div className='absolute top-0 left-1/4 w-px h-full bg-gradient-to-b from-transparent via-[#00C77E]/30 to-transparent animate-matrix-rain'></div>
        <div className='absolute top-0 left-3/4 w-px h-full bg-gradient-to-b from-transparent via-[#00C77E]/20 to-transparent animate-matrix-rain animation-delay-2000'></div>
        <div className='absolute top-0 left-1/2 w-px h-full bg-gradient-to-b from-transparent via-[#FF2E4D]/20 to-transparent animate-matrix-rain animation-delay-4000'></div>
        
        {/* Floating Particles */}
        <div className='absolute top-1/4 left-1/6 w-1 h-1 bg-[#00C77E] rounded-full animate-particle-float shadow-[0_0_10px_#00C77E]'></div>
        <div className='absolute top-1/3 right-1/4 w-1 h-1 bg-[#00C77E] rounded-full animate-particle-float animation-delay-1000 shadow-[0_0_10px_#00C77E]'></div>
        <div className='absolute bottom-1/4 left-1/3 w-1 h-1 bg-[#FF2E4D] rounded-full animate-particle-float animation-delay-2000 shadow-[0_0_10px_#FF2E4D]'></div>
        <div className='absolute top-1/2 right-1/6 w-1 h-1 bg-[#00C77E] rounded-full animate-particle-float animation-delay-3000 shadow-[0_0_10px_#00C77E]'></div>
        <div className='absolute top-3/4 left-1/5 w-1 h-1 bg-[#FF2E4D] rounded-full animate-particle-float animation-delay-4000 shadow-[0_0_10px_#FF2E4D]'></div>
        <div className='absolute top-1/5 right-1/3 w-1 h-1 bg-[#00C77E] rounded-full animate-particle-float animation-delay-500 shadow-[0_0_10px_#00C77E]'></div>
      </div>
      
      {/* Footer */}
      <footer className='absolute bottom-0 left-0 right-0 z-30 p-6'>
        <div className='flex justify-between items-center text-[#B0B0B0] text-sm'>
          <div className='flex space-x-6'>
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
