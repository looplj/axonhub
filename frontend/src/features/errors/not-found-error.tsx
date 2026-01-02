import { useState, useMemo } from 'react';
import { useNavigate, useRouter, useLocation } from '@tanstack/react-router';
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
  IconExternalLink,
} from '@tabler/icons-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';

interface SuggestedPage {
  title: string;
  description: string;
  path: string;
  icon: React.ReactNode;
  keywords: string[];
}

export default function NotFoundError() {
  const navigate = useNavigate();
  const { history } = useRouter();
  const location = useLocation();
  const [searchQuery, setSearchQuery] = useState('');

  const suggestedPages: SuggestedPage[] = [
    {
      title: 'Dashboard',
      description: 'Overview of your AxonHub instance',
      path: '/',
      icon: <IconHome className='h-5 w-5' />,
      keywords: ['dashboard', 'home', 'overview', 'main'],
    },
    {
      title: 'Channels',
      description: 'Manage AI model channels and configurations',
      path: '/channels',
      icon: <IconMessages className='h-5 w-5' />,
      keywords: ['channels', 'models', 'ai', 'configuration', 'chat'],
    },
    {
      title: 'Requests',
      description: 'Monitor API requests and usage analytics',
      path: '/requests',
      icon: <IconChartBar className='h-5 w-5' />,
      keywords: ['requests', 'api', 'analytics', 'monitoring', 'usage'],
    },
    {
      title: 'Users',
      description: 'User management and permissions',
      path: '/users',
      icon: <IconUsers className='h-5 w-5' />,
      keywords: ['users', 'people', 'accounts', 'management'],
    },
    {
      title: 'API Keys',
      description: 'Generate and manage API authentication keys',
      path: '/api-keys',
      icon: <IconKey className='h-5 w-5' />,
      keywords: ['api', 'keys', 'authentication', 'tokens', 'access'],
    },
    {
      title: 'Roles',
      description: 'Configure user roles and permissions',
      path: '/roles',
      icon: <IconShield className='h-5 w-5' />,
      keywords: ['roles', 'permissions', 'access', 'security', 'rbac'],
    },
    {
      title: 'Playground',
      description: 'Test and experiment with AI models',
      path: '/playground',
      icon: <IconPlayerPlay className='h-5 w-5' />,
      keywords: ['playground', 'test', 'experiment', 'try', 'demo'],
    },
    {
      title: 'Settings',
      description: 'System configuration and preferences',
      path: '/settings',
      icon: <IconSettings className='h-5 w-5' />,
      keywords: ['settings', 'configuration', 'preferences', 'system'],
    },
    {
      title: 'Help Center',
      description: 'Documentation and support resources',
      path: '/help-center',
      icon: <IconHelpCircle className='h-5 w-5' />,
      keywords: ['help', 'documentation', 'support', 'guide', 'docs'],
    },
  ];

  // Smart suggestions based on current URL and search query
  const smartSuggestions = useMemo(() => {
    const currentPath = location.pathname.toLowerCase();
    const query = searchQuery.toLowerCase();

    // Score pages based on URL similarity and search relevance
    const scoredPages = suggestedPages.map((page) => {
      let score = 0;

      // URL path similarity
      const pathSegments = currentPath.split('/').filter(Boolean);
      const pageSegments = page.path.split('/').filter(Boolean);

      pathSegments.forEach((segment) => {
        if (page.path.includes(segment) || page.keywords.some((k) => k.includes(segment))) {
          score += 3;
        }
      });

      // Search query relevance
      if (query) {
        if (page.title.toLowerCase().includes(query)) score += 5;
        if (page.description.toLowerCase().includes(query)) score += 3;
        page.keywords.forEach((keyword) => {
          if (keyword.includes(query)) score += 2;
        });
      }

      return { ...page, score };
    });

    // Sort by score and return top suggestions
    return scoredPages.sort((a, b) => b.score - a.score).slice(0, query ? 6 : 4);
  }, [location.pathname, searchQuery, suggestedPages]);

  const handlePageNavigation = (path: string) => {
    navigate({ to: path });
  };

  return (
    <div className='from-background via-background to-muted/20 min-h-svh bg-gradient-to-br'>
      <div className='container mx-auto px-4 py-16'>
        <div className='mx-auto max-w-4xl'>
          {/* Header Section */}
          <div className='mb-12 text-center'>
            <div className='relative'>
              <h1 className='text-primary/10 text-[8rem] leading-none font-bold select-none md:text-[12rem]'>404</h1>
              <div className='absolute inset-0 flex items-center justify-center'>
                <div className='text-center'>
                  <h2 className='text-foreground mb-4 text-3xl font-bold md:text-4xl'>Page Not Found</h2>
                  <p className='text-muted-foreground mx-auto max-w-md text-lg'>
                    The page you're looking for doesn't exist, but we can help you find what you need.
                  </p>
                </div>
              </div>
            </div>
          </div>

          {/* Search Section */}
          <Card className='border-primary/20 mb-8 border-2 border-dashed'>
            <CardContent className='p-6'>
              <div className='mb-4 flex items-center gap-3'>
                <IconSearch className='text-primary h-5 w-5' />
                <h3 className='text-lg font-semibold'>Find what you're looking for</h3>
              </div>
              <div className='relative'>
                <Input
                  placeholder='Search for pages, features, or functionality...'
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className='pl-10'
                />
                <IconSearch className='text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 transform' />
              </div>
            </CardContent>
          </Card>

          {/* Suggested Pages */}
          <div className='mb-8'>
            <h3 className='mb-6 flex items-center gap-2 text-xl font-semibold'>
              <IconExternalLink className='text-primary h-5 w-5' />
              {searchQuery ? 'Search Results' : 'Suggested Pages'}
            </h3>
            <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
              {smartSuggestions.map((page) => (
                <Card
                  key={page.path}
                  className='hover:border-primary/50 cursor-pointer border transition-all duration-200 hover:scale-[1.02] hover:shadow-lg'
                  onClick={() => handlePageNavigation(page.path)}
                >
                  <CardContent className='p-4'>
                    <div className='flex items-start gap-3'>
                      <div className='bg-primary/10 text-primary flex-shrink-0 rounded-lg p-2'>{page.icon}</div>
                      <div className='min-w-0 flex-1'>
                        <h4 className='text-foreground mb-1 truncate font-semibold'>{page.title}</h4>
                        <p className='text-muted-foreground line-clamp-2 text-sm'>{page.description}</p>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          </div>

          {/* Action Buttons */}
          <div className='flex flex-col items-center justify-center gap-4 sm:flex-row'>
            <Button variant='outline' onClick={() => history.go(-1)} className='flex min-w-[140px] items-center gap-2'>
              <IconArrowLeft className='h-4 w-4' />
              Go Back
            </Button>
            <Button onClick={() => navigate({ to: '/' })} className='flex min-w-[140px] items-center gap-2'>
              <IconHome className='h-4 w-4' />
              Dashboard
            </Button>
          </div>

          {/* Additional Help */}
          <div className='mt-12 text-center'>
            <p className='text-muted-foreground mb-4 text-sm'>Still can't find what you're looking for?</p>
            <Button variant='ghost' onClick={() => navigate({ to: '/help-center' })} className='text-primary hover:text-primary/80'>
              Visit Help Center →
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
