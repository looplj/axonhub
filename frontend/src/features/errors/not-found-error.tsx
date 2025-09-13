import { useNavigate, useRouter, useLocation } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { 
  IconHome, 
  IconSearch, 
  IconUsers, 
  IconKey, 
  IconMessages, 
  IconSettings,
  IconChartBar,
  IconShield,
  IconPlayerPlay,
  IconHelpCircle,
  IconArrowLeft,
  IconExternalLink
} from '@tabler/icons-react'
import { useState, useMemo } from 'react'

interface SuggestedPage {
  title: string
  description: string
  path: string
  icon: React.ReactNode
  keywords: string[]
}

export default function NotFoundError() {
  const navigate = useNavigate()
  const { history } = useRouter()
  const location = useLocation()
  const [searchQuery, setSearchQuery] = useState('')

  const suggestedPages: SuggestedPage[] = [
    {
      title: 'Dashboard',
      description: 'Overview of your AxonHub instance',
      path: '/',
      icon: <IconHome className="w-5 h-5" />,
      keywords: ['dashboard', 'home', 'overview', 'main']
    },
    {
      title: 'Channels',
      description: 'Manage AI model channels and configurations',
      path: '/channels',
      icon: <IconMessages className="w-5 h-5" />,
      keywords: ['channels', 'models', 'ai', 'configuration', 'chat']
    },
    {
      title: 'Requests',
      description: 'Monitor API requests and usage analytics',
      path: '/requests',
      icon: <IconChartBar className="w-5 h-5" />,
      keywords: ['requests', 'api', 'analytics', 'monitoring', 'usage']
    },
    {
      title: 'Users',
      description: 'User management and permissions',
      path: '/users',
      icon: <IconUsers className="w-5 h-5" />,
      keywords: ['users', 'people', 'accounts', 'management']
    },
    {
      title: 'API Keys',
      description: 'Generate and manage API authentication keys',
      path: '/api-keys',
      icon: <IconKey className="w-5 h-5" />,
      keywords: ['api', 'keys', 'authentication', 'tokens', 'access']
    },
    {
      title: 'Roles',
      description: 'Configure user roles and permissions',
      path: '/roles',
      icon: <IconShield className="w-5 h-5" />,
      keywords: ['roles', 'permissions', 'access', 'security', 'rbac']
    },
    {
      title: 'Playground',
      description: 'Test and experiment with AI models',
      path: '/playground',
      icon: <IconPlayerPlay className="w-5 h-5" />,
      keywords: ['playground', 'test', 'experiment', 'try', 'demo']
    },
    {
      title: 'Settings',
      description: 'System configuration and preferences',
      path: '/settings',
      icon: <IconSettings className="w-5 h-5" />,
      keywords: ['settings', 'configuration', 'preferences', 'system']
    },
    {
      title: 'Help Center',
      description: 'Documentation and support resources',
      path: '/help-center',
      icon: <IconHelpCircle className="w-5 h-5" />,
      keywords: ['help', 'documentation', 'support', 'guide', 'docs']
    }
  ]

  // Smart suggestions based on current URL and search query
  const smartSuggestions = useMemo(() => {
    const currentPath = location.pathname.toLowerCase()
    const query = searchQuery.toLowerCase()
    
    // Score pages based on URL similarity and search relevance
    const scoredPages = suggestedPages.map(page => {
      let score = 0
      
      // URL path similarity
      const pathSegments = currentPath.split('/').filter(Boolean)
      const pageSegments = page.path.split('/').filter(Boolean)
      
      pathSegments.forEach(segment => {
        if (page.path.includes(segment) || page.keywords.some(k => k.includes(segment))) {
          score += 3
        }
      })
      
      // Search query relevance
      if (query) {
        if (page.title.toLowerCase().includes(query)) score += 5
        if (page.description.toLowerCase().includes(query)) score += 3
        page.keywords.forEach(keyword => {
          if (keyword.includes(query)) score += 2
        })
      }
      
      return { ...page, score }
    })
    
    // Sort by score and return top suggestions
    return scoredPages
      .sort((a, b) => b.score - a.score)
      .slice(0, query ? 6 : 4)
  }, [location.pathname, searchQuery, suggestedPages])

  const handlePageNavigation = (path: string) => {
    navigate({ to: path })
  }

  return (
    <div className='min-h-svh bg-gradient-to-br from-background via-background to-muted/20'>
      <div className='container mx-auto px-4 py-16'>
        <div className='max-w-4xl mx-auto'>
          {/* Header Section */}
          <div className='text-center mb-12'>
            <div className='relative'>
              <h1 className='text-[8rem] md:text-[12rem] font-bold text-primary/10 leading-none select-none'>
                404
              </h1>
              <div className='absolute inset-0 flex items-center justify-center'>
                <div className='text-center'>
                  <h2 className='text-3xl md:text-4xl font-bold text-foreground mb-4'>
                    Page Not Found
                  </h2>
                  <p className='text-lg text-muted-foreground max-w-md mx-auto'>
                    The page you're looking for doesn't exist, but we can help you find what you need.
                  </p>
                </div>
              </div>
            </div>
          </div>

          {/* Search Section */}
          <Card className='mb-8 border-2 border-dashed border-primary/20'>
            <CardContent className='p-6'>
              <div className='flex items-center gap-3 mb-4'>
                <IconSearch className='w-5 h-5 text-primary' />
                <h3 className='text-lg font-semibold'>Find what you're looking for</h3>
              </div>
              <div className='relative'>
                <Input
                  placeholder='Search for pages, features, or functionality...'
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className='pl-10'
                />
                <IconSearch className='absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-muted-foreground' />
              </div>
            </CardContent>
          </Card>

          {/* Suggested Pages */}
          <div className='mb-8'>
            <h3 className='text-xl font-semibold mb-6 flex items-center gap-2'>
              <IconExternalLink className='w-5 h-5 text-primary' />
              {searchQuery ? 'Search Results' : 'Suggested Pages'}
            </h3>
            <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
              {smartSuggestions.map((page) => (
                <Card 
                  key={page.path}
                  className='cursor-pointer transition-all duration-200 hover:shadow-lg hover:scale-[1.02] border hover:border-primary/50'
                  onClick={() => handlePageNavigation(page.path)}
                >
                  <CardContent className='p-4'>
                    <div className='flex items-start gap-3'>
                      <div className='p-2 rounded-lg bg-primary/10 text-primary flex-shrink-0'>
                        {page.icon}
                      </div>
                      <div className='flex-1 min-w-0'>
                        <h4 className='font-semibold text-foreground mb-1 truncate'>
                          {page.title}
                        </h4>
                        <p className='text-sm text-muted-foreground line-clamp-2'>
                          {page.description}
                        </p>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          </div>

          {/* Action Buttons */}
          <div className='flex flex-col sm:flex-row gap-4 justify-center items-center'>
            <Button 
              variant='outline' 
              onClick={() => history.go(-1)}
              className='flex items-center gap-2 min-w-[140px]'
            >
              <IconArrowLeft className='w-4 h-4' />
              Go Back
            </Button>
            <Button 
              onClick={() => navigate({ to: '/' })}
              className='flex items-center gap-2 min-w-[140px]'
            >
              <IconHome className='w-4 h-4' />
              Dashboard
            </Button>
          </div>

          {/* Additional Help */}
          <div className='mt-12 text-center'>
            <p className='text-sm text-muted-foreground mb-4'>
              Still can't find what you're looking for?
            </p>
            <Button 
              variant='ghost' 
              onClick={() => navigate({ to: '/help-center' })}
              className='text-primary hover:text-primary/80'
            >
              Visit Help Center →
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
