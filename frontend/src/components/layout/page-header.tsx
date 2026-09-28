import React from 'react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import styles from './page-header.module.css';

type PageHeaderDescription = string | readonly string[];

interface PageHeaderProps extends Omit<React.HTMLAttributes<HTMLElement>, 'title'> {
  title: string;
  /**
   * Plain text description. Pass an array to render multiple paragraphs, which
   * are measured and collapsed together.
   */
  description?: PageHeaderDescription;
  /** Primary page actions, usually a `*-primary-buttons` component. */
  actions?: React.ReactNode;
  /** Supplementary content that must stay visible, e.g. model catalog status. */
  metadata?: React.ReactNode;
  /**
   * How the action row handles limited width:
   * - `wrap` (default): buttons wrap onto additional rows.
   * - `scroll`: actions keep a single row and scroll horizontally.
   */
  actionLayout?: 'wrap' | 'scroll';
}

const COLLAPSED_LINE_COUNT = 2;

function toParagraphs(description?: PageHeaderDescription): string[] {
  if (!description) return [];
  const paragraphs = typeof description === 'string' ? [description] : [...description];
  return paragraphs.filter((paragraph) => paragraph.trim() !== '');
}

/**
 * Responsive page header for list-style pages.
 *
 * The header takes part in normal layout flow and sizes itself to its content,
 * so a wrapping description or action row always pushes the page body down
 * instead of overlapping it. Width-based decisions use container queries on the
 * header itself, so collapsing the sidebar is reflected immediately.
 */
export function PageHeader({
  title,
  description,
  actions,
  metadata,
  actionLayout = 'wrap',
  className,
  ...props
}: PageHeaderProps) {
  const { t } = useTranslation();

  const paragraphs = React.useMemo(() => toParagraphs(description), [description]);
  const hasDescription = paragraphs.length > 0;

  const descriptionId = React.useId();
  const probeRef = React.useRef<HTMLDivElement>(null);
  const frameRef = React.useRef<number | null>(null);

  const [expanded, setExpanded] = React.useState(false);
  const [isOverflowing, setIsOverflowing] = React.useState(false);

  // Reset the disclosure when the text changes so a different description never
  // inherits the previous expanded state.
  const descriptionKey = paragraphs.join('\u0000');
  React.useEffect(() => {
    setExpanded(false);
  }, [descriptionKey]);

  React.useEffect(() => {
    const probe = probeRef.current;
    if (!probe) return;

    const measure = () => {
      frameRef.current = null;

      // Read the line height from an actual rendered paragraph so the collapsed
      // height always matches the visible text, even if a theme overrides the
      // typography scale.
      const line = probe.querySelector('p') ?? probe;
      const lineHeight = Number.parseFloat(window.getComputedStyle(line).lineHeight);
      if (!Number.isFinite(lineHeight) || lineHeight <= 0) return;

      const collapsedHeight = lineHeight * COLLAPSED_LINE_COUNT;
      // 1px tolerance absorbs sub-pixel line-height rounding.
      const overflowing = probe.getBoundingClientRect().height > collapsedHeight + 1;
      setIsOverflowing((previous) => (previous === overflowing ? previous : overflowing));
    };

    const scheduleMeasure = () => {
      if (frameRef.current !== null) return;
      frameRef.current = window.requestAnimationFrame(measure);
    };

    const observer = new ResizeObserver(scheduleMeasure);
    observer.observe(probe);

    scheduleMeasure();

    // Web fonts can change the measured height after the first paint.
    let cancelled = false;

    const fontsReady = document.fonts?.ready;

    if (fontsReady) {
      fontsReady
        .then(() => {
          if (!cancelled) scheduleMeasure();
        })
        .catch(() => {
          // Font loading failures must not break the header.
        });
    }

    return () => {
      cancelled = true;
      observer.disconnect();
      if (frameRef.current !== null) {
        window.cancelAnimationFrame(frameRef.current);
        frameRef.current = null;
      }
    };
  }, [descriptionKey]);

  const renderParagraphs = () =>
    paragraphs.map((paragraph, index) => (
      // The text itself is the stable identity here; duplicates are unlikely and
      // harmless for a purely presentational list.
      <p key={`${index}-${paragraph}`} className={styles.line}>
        {paragraph}
      </p>
    ));

  return (
    <header
      className={cn(styles.root, 'responsive-page-header', className)}
      data-testid='page-header'
      data-overflowing={hasDescription && isOverflowing}
      {...props}
    >
      <div className={styles.layout}>
        <div className={styles.text}>
          <h2 className={cn(styles.title, 'text-xl font-bold tracking-tight')}>{title}</h2>

          {hasDescription && (
            <div className={styles.descriptionArea}>
              <div
                id={descriptionId}
                className={cn(styles.description, 'text-sm text-muted-foreground')}
                data-testid='page-header-description'
                data-expanded={expanded}
              >
                {renderParagraphs()}
              </div>

              <div className={cn(styles.probe, 'text-sm text-muted-foreground')} aria-hidden='true'>
                <div ref={probeRef} className={styles.probeInner}>
                  {renderParagraphs()}
                </div>
              </div>
            </div>
          )}

          {hasDescription && (
            <button
              type='button'
              className={styles.toggle}
              data-testid='page-header-description-toggle'
              data-overflowing={isOverflowing}
              aria-controls={descriptionId}
              aria-expanded={expanded}
              onClick={() => setExpanded((previous) => !previous)}
            >
              {expanded ? t('common.pageHeader.collapseDescription') : t('common.pageHeader.expandDescription')}
            </button>
          )}

          {metadata && (
            <div className={styles.metadata} data-testid='page-header-metadata'>
              {metadata}
            </div>
          )}
        </div>

        {actions && (
          <div
            className={cn(styles.actions, actionLayout === 'scroll' ? styles.actionsScroll : styles.actionsWrap)}
            data-testid='page-header-actions'
            data-layout={actionLayout}
          >
            {actions}
          </div>
        )}
      </div>
    </header>
  );
}

PageHeader.displayName = 'PageHeader';
