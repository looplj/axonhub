import { useState } from 'react';
import { cn } from '@/lib/utils';
import { AxonHubLogo } from '@/components/axonhub-logo';

interface BrandLogoProps {
  brandLogo?: string | null;
  className?: string;
}

// Custom brand logo when configured, otherwise (or when it fails to load) the theme-aware AxonHub mark.
export function BrandLogo({ brandLogo, className }: BrandLogoProps) {
  const [failedSrc, setFailedSrc] = useState<string | null>(null);

  if (brandLogo && brandLogo !== failedSrc) {
    return <img src={brandLogo} alt='Brand Logo' className={cn('size-8 object-cover', className)} onError={() => setFailedSrc(brandLogo)} />;
  }

  return <AxonHubLogo className={cn('text-foreground size-7', className)} />;
}
