interface Props {
  children: React.ReactNode
}

export default function AuthLayout({ children }: Props) {
  return (
    <div className='relative min-h-screen overflow-hidden'>
      {/* Darker animated gradient background */}
      <div className='absolute inset-0 bg-gradient-to-br from-gray-950 via-slate-950 to-black'>
        <div className='absolute inset-0 bg-gradient-to-tr from-transparent via-blue-600/15 to-transparent animate-pulse'></div>
        <div className='absolute inset-0 bg-gradient-to-bl from-transparent via-cyan-600/10 to-transparent animate-pulse animation-delay-1000'></div>
        
        {/* Main animated blobs - larger and more vibrant */}
        <div className='absolute -top-20 left-1/4 w-96 h-96 bg-blue-500/30 rounded-full blur-3xl animate-blob'></div>
        <div className='absolute -top-10 right-1/4 w-80 h-80 bg-cyan-400/35 rounded-full blur-3xl animate-blob animation-delay-2000'></div>
        <div className='absolute -bottom-20 left-1/3 w-96 h-96 bg-teal-500/25 rounded-full blur-3xl animate-blob animation-delay-4000'></div>
        <div className='absolute bottom-1/4 right-1/5 w-72 h-72 bg-purple-500/20 rounded-full blur-3xl animate-blob animation-delay-3000'></div>
        
        {/* Additional smaller animated elements */}
        <div className='absolute top-1/3 left-1/6 w-48 h-48 bg-indigo-400/25 rounded-full blur-2xl animate-blob animation-delay-1500'></div>
        <div className='absolute bottom-1/3 right-1/3 w-56 h-56 bg-blue-400/30 rounded-full blur-2xl animate-blob animation-delay-5000'></div>
        
        {/* Moving light streaks */}
        <div className='absolute top-0 left-0 w-full h-1 bg-gradient-to-r from-transparent via-cyan-400/50 to-transparent animate-slide-right'></div>
        <div className='absolute bottom-0 right-0 w-full h-1 bg-gradient-to-l from-transparent via-blue-400/50 to-transparent animate-slide-left animation-delay-2000'></div>
        <div className='absolute top-1/2 left-0 w-1 h-full bg-gradient-to-b from-transparent via-teal-400/40 to-transparent animate-slide-down animation-delay-3000'></div>
      </div>
      
      {/* Content container */}
      <div className='relative z-10 container grid h-svh max-w-none items-center justify-center'>
        <div className='mx-auto flex w-full flex-col justify-center space-y-2 py-8 sm:w-[480px] sm:p-8 animate-fade-in-up'>
          <div className='mb-8 flex items-center justify-center'>
            <div className='flex items-center space-x-3 animate-fade-in'>
              <div className='relative'>
                <svg
                  xmlns='http://www.w3.org/2000/svg'
                  viewBox='0 0 24 24'
                  fill='none'
                  stroke='currentColor'
                  strokeWidth='2'
                  strokeLinecap='round'
                  strokeLinejoin='round'
                  className='h-8 w-8 text-cyan-400 animate-glow'
                >
                  <path d='M15 6v12a3 3 0 1 0 3-3H6a3 3 0 1 0 3 3V6a3 3 0 1 0-3 3h12a3 3 0 1 0-3-3' />
                </svg>
                <div className='absolute inset-0 h-8 w-8 bg-cyan-400/20 rounded-full blur-md animate-pulse'></div>
              </div>
              <h1 className='text-3xl font-bold bg-gradient-to-r from-white to-cyan-200 bg-clip-text text-transparent'>
                AxonHub
              </h1>
            </div>
          </div>
          <div className='animate-fade-in-up animation-delay-300'>
            {children}
          </div>
        </div>
      </div>
      
      {/* Enhanced floating particles and light effects */}
      <div className='absolute inset-0 overflow-hidden pointer-events-none'>
        {/* Floating particles */}
        <div className='absolute top-1/4 left-1/4 w-3 h-3 bg-cyan-400/50 rounded-full animate-float shadow-lg shadow-cyan-400/30'></div>
        <div className='absolute top-1/3 right-1/3 w-2 h-2 bg-blue-400/60 rounded-full animate-float animation-delay-1000 shadow-lg shadow-blue-400/40'></div>
        <div className='absolute bottom-1/4 left-1/2 w-2.5 h-2.5 bg-teal-400/50 rounded-full animate-float animation-delay-2000 shadow-lg shadow-teal-400/30'></div>
        <div className='absolute top-1/2 right-1/4 w-2 h-2 bg-cyan-300/60 rounded-full animate-float animation-delay-3000 shadow-lg shadow-cyan-300/40'></div>
        <div className='absolute top-3/4 left-1/5 w-1.5 h-1.5 bg-purple-400/50 rounded-full animate-float animation-delay-4000 shadow-lg shadow-purple-400/30'></div>
        <div className='absolute top-1/5 right-1/2 w-2 h-2 bg-indigo-400/55 rounded-full animate-float animation-delay-500 shadow-lg shadow-indigo-400/35'></div>
        
        {/* Glowing orbs */}
        <div className='absolute top-1/6 left-3/4 w-4 h-4 bg-cyan-300/40 rounded-full animate-pulse-glow animation-delay-1000'></div>
        <div className='absolute bottom-1/6 right-1/6 w-3 h-3 bg-blue-300/45 rounded-full animate-pulse-glow animation-delay-2500'></div>
        <div className='absolute top-2/3 left-1/8 w-3.5 h-3.5 bg-teal-300/40 rounded-full animate-pulse-glow animation-delay-4500'></div>
        
        {/* Shooting stars */}
        <div className='absolute top-1/4 -left-10 w-20 h-0.5 bg-gradient-to-r from-transparent via-cyan-400/70 to-transparent animate-shooting-star animation-delay-6000'></div>
        <div className='absolute top-3/4 -right-10 w-16 h-0.5 bg-gradient-to-l from-transparent via-blue-400/60 to-transparent animate-shooting-star-reverse animation-delay-8000'></div>
      </div>
    </div>
  )
}
