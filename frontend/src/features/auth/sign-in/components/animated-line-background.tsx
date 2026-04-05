import { type FC, useCallback, useEffect, useRef } from 'react';
import type { FormBounds, MouseArea, Particle } from './animated-line-background.engine';
import {
  animationConfig,
  getFormBounds,
  initParticles,
  renderParticles,
  updateParticles,
} from './animated-line-background.engine';

interface AnimationDiagnosticsSnapshot {
  targetFps: number;
  frameIntervalMs: number;
  maxCatchUpMs: number;
  maxStepsPerFrame: number;
  frameCount: number;
  simulationStepCount: number;
  renderCount: number;
  simulatedMs: number;
  accumulatorMs: number;
  lastFrameDeltaMs: number;
  lastClampedDeltaMs: number;
  lastFrameStepCount: number;
  lastAppliedDeltaMs: number;
  particleChecksum: number;
}

interface AnimationDiagnostics {
  reset(): void;
  snapshot(): AnimationDiagnosticsSnapshot;
  simulate(stepMs: number, steps: number): void;
  simulateLargeGap(deltaMs: number): void;
}

declare global {
  interface Window {
    __AXONHUB_SIGNIN_ANIMATION__?: AnimationDiagnostics;
  }
}

const DEBUG_QUERY_PARAM = '__axonhub_debug_animation';

const createMouseArea = (): MouseArea => ({ x: null, y: null, max: 20000 });

const cloneParticles = (particles: Particle[]): Particle[] => particles.map((particle) => ({ ...particle }));

const getParticleChecksum = (particles: Particle[]): number => {
  return particles.reduce((checksum, particle, index) => {
    const x = Math.round(particle.x * 1000);
    const y = Math.round(particle.y * 1000);
    const factor = index + 1;

    return checksum + x * factor * 31 + y * factor * 17;
  }, 0);
};

const shouldExposeAnimationDiagnostics = (): boolean => {
  if (!import.meta.env.DEV || typeof window === 'undefined') {
    return false;
  }

  return new URLSearchParams(window.location.search).get(DEBUG_QUERY_PARAM) === '1';
};

const AnimatedLineBackground: FC = () => {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const ctxRef = useRef<CanvasRenderingContext2D | null>(null);
  const animationRef = useRef<number | null>(null);
  const particlesRef = useRef<Particle[]>([]);
  const diagnosticsInitialParticlesRef = useRef<Particle[] | null>(null);
  const diagnosticsCanvasSizeRef = useRef<{ width: number; height: number } | null>(null);
  const formBoundsRef = useRef<FormBounds | null>(null);
  const mouseAreaRef = useRef<MouseArea>(createMouseArea());
  const frameCountRef = useRef(0);
  const simulationStepCountRef = useRef(0);
  const renderCountRef = useRef(0);
  const simulatedMsRef = useRef(0);
  const accumulatorRef = useRef(0);
  const lastTimestampRef = useRef<number | null>(null);
  const lastFrameDeltaMsRef = useRef(0);
  const lastClampedDeltaMsRef = useRef(0);
  const lastFrameStepCountRef = useRef(0);
  const lastAppliedDeltaMsRef = useRef(0);
  const lastRenderedMouseAreaRef = useRef<MouseArea>(createMouseArea());
  const lastRenderedFormBoundsRef = useRef<FormBounds | null>(null);

  const getMeasuredFormBounds = useCallback((): FormBounds | null => {
    const el = document.getElementById('auth-card-wrapper');
    if (!el) return null;
    const rect = el.getBoundingClientRect();
    return {
      formCenterX: rect.left + rect.width / 2,
      formCenterY: rect.top + rect.height / 2,
      formLeft: rect.left,
      formRight: rect.right,
      formTop: rect.top,
      formBottom: rect.bottom,
    };
  }, []);

  const updateFormBounds = useCallback((canvasWidth: number, canvasHeight: number) => {
    const newBounds = getMeasuredFormBounds() ?? getFormBounds(canvasWidth, canvasHeight);
    const prev = formBoundsRef.current;
    
    if (!prev || 
        prev.formLeft !== newBounds.formLeft || 
        prev.formRight !== newBounds.formRight || 
        prev.formTop !== newBounds.formTop || 
        prev.formBottom !== newBounds.formBottom) {
      formBoundsRef.current = newBounds;
      return true;
    }
    return false;
  }, [getMeasuredFormBounds]);

  const resize = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return { sizeChanged: false, boundsChanged: false };

    const prevWidth = canvas.width;
    const prevHeight = canvas.height;

    canvas.width = window.innerWidth;
    canvas.height = window.innerHeight;

    ctxRef.current = ctxRef.current ?? canvas.getContext('2d');
    const boundsChanged = updateFormBounds(canvas.width, canvas.height);
    const sizeChanged = prevWidth !== canvas.width || prevHeight !== canvas.height;

    return { sizeChanged, boundsChanged };
  }, [updateFormBounds]);

  const handleResize = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const { sizeChanged } = resize();

    if (sizeChanged || particlesRef.current.length === 0) {
      const formBounds = formBoundsRef.current ?? getFormBounds(canvas.width, canvas.height);
      particlesRef.current = initParticles(canvas.width, canvas.height, formBounds);
    }
  }, [resize]);

  const resetFrameTimingState = useCallback(() => {
    accumulatorRef.current = 0;
    lastTimestampRef.current = null;
    lastFrameDeltaMsRef.current = 0;
    lastClampedDeltaMsRef.current = 0;
    lastFrameStepCountRef.current = 0;
    lastAppliedDeltaMsRef.current = 0;
  }, []);

  const renderFrame = useCallback(() => {
    const canvas = canvasRef.current;
    const ctx = ctxRef.current;
    const formBounds = formBoundsRef.current;
    if (!canvas || !ctx || !formBounds) return;

    renderParticles(ctx, canvas.width, canvas.height, particlesRef.current, mouseAreaRef.current, formBounds);

    renderCountRef.current += 1;
  }, []);

  const applyAnimationStep = useCallback((deltaMs: number) => {
    const canvas = canvasRef.current;
    const formBounds = formBoundsRef.current;
    if (!canvas || !formBounds) return;

    updateParticles(canvas.width, canvas.height, particlesRef.current, mouseAreaRef.current, formBounds);

    simulationStepCountRef.current += 1;
    simulatedMsRef.current += deltaMs;
    lastAppliedDeltaMsRef.current = deltaMs;
  }, []);

  const processAnimationFrame = useCallback(
    (deltaMs: number) => {
      const safeDeltaMs = Number.isFinite(deltaMs) ? Math.max(0, deltaMs) : 0;
      const clampedDeltaMs = Math.min(safeDeltaMs, animationConfig.maxCatchUpMs);

      lastFrameDeltaMsRef.current = safeDeltaMs;
      lastClampedDeltaMsRef.current = clampedDeltaMs;
      accumulatorRef.current += clampedDeltaMs;

      let steps = 0;
      while (accumulatorRef.current >= animationConfig.frameIntervalMs && steps < animationConfig.maxStepsPerFrame) {
        accumulatorRef.current -= animationConfig.frameIntervalMs;
        applyAnimationStep(animationConfig.frameIntervalMs);
        steps += 1;
      }

      lastFrameStepCountRef.current = steps;
      if (steps === 0) {
        lastAppliedDeltaMsRef.current = 0;
      }

      const mouseMoved =
        lastRenderedMouseAreaRef.current.x !== mouseAreaRef.current.x ||
        lastRenderedMouseAreaRef.current.y !== mouseAreaRef.current.y;
      const formBoundsChanged = lastRenderedFormBoundsRef.current !== formBoundsRef.current;

      if (steps > 0 || mouseMoved || formBoundsChanged) {
        renderFrame();
        lastRenderedMouseAreaRef.current.x = mouseAreaRef.current.x;
        lastRenderedMouseAreaRef.current.y = mouseAreaRef.current.y;
        lastRenderedFormBoundsRef.current = formBoundsRef.current;
      }
    },
    [applyAnimationStep, renderFrame]
  );

  const resetDiagnosticsState = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    frameCountRef.current = 0;
    simulationStepCountRef.current = 0;
    renderCountRef.current = 0;
    simulatedMsRef.current = 0;
    resetFrameTimingState();
    mouseAreaRef.current = createMouseArea();

    const { sizeChanged, boundsChanged } = resize();

    const nextCanvasSize = { width: canvas.width, height: canvas.height };
    const needsNewInitialParticles =
      sizeChanged ||
      boundsChanged ||
      diagnosticsInitialParticlesRef.current === null ||
      diagnosticsCanvasSizeRef.current?.width !== nextCanvasSize.width ||
      diagnosticsCanvasSizeRef.current?.height !== nextCanvasSize.height;

    if (needsNewInitialParticles) {
      const formBounds = formBoundsRef.current ?? getFormBounds(nextCanvasSize.width, nextCanvasSize.height);
      diagnosticsInitialParticlesRef.current = initParticles(nextCanvasSize.width, nextCanvasSize.height, formBounds);
      diagnosticsCanvasSizeRef.current = nextCanvasSize;
    }

    particlesRef.current = cloneParticles(diagnosticsInitialParticlesRef.current);
    renderFrame();
    renderCountRef.current = 0;
  }, [renderFrame, resize, resetFrameTimingState]);

  const snapshotDiagnostics = useCallback<() => AnimationDiagnosticsSnapshot>(() => {
    return {
      targetFps: animationConfig.targetFps,
      frameIntervalMs: animationConfig.frameIntervalMs,
      maxCatchUpMs: animationConfig.maxCatchUpMs,
      maxStepsPerFrame: animationConfig.maxStepsPerFrame,
      frameCount: frameCountRef.current,
      simulationStepCount: simulationStepCountRef.current,
      renderCount: renderCountRef.current,
      simulatedMs: simulatedMsRef.current,
      accumulatorMs: accumulatorRef.current,
      lastFrameDeltaMs: lastFrameDeltaMsRef.current,
      lastClampedDeltaMs: lastClampedDeltaMsRef.current,
      lastFrameStepCount: lastFrameStepCountRef.current,
      lastAppliedDeltaMs: lastAppliedDeltaMsRef.current,
      particleChecksum: getParticleChecksum(particlesRef.current),
    };
  }, []);

  const simulateDiagnostics = useCallback(
    (stepMs: number, steps: number) => {
      const safeStepMs = Number.isFinite(stepMs) ? stepMs : 0;
      const safeSteps = Number.isFinite(steps) ? Math.max(0, Math.floor(steps)) : 0;

      for (let index = 0; index < safeSteps; index += 1) {
        frameCountRef.current += 1;
        processAnimationFrame(safeStepMs);
      }
    },
    [processAnimationFrame]
  );

  const simulateLargeGapDiagnostics = useCallback(
    (deltaMs: number) => {
      const safeDeltaMs = Number.isFinite(deltaMs) ? Math.max(0, deltaMs) : 0;

      frameCountRef.current += 1;
      processAnimationFrame(safeDeltaMs);
    },
    [processAnimationFrame]
  );

  const animate = useCallback(
    (timestamp: number) => {
      if (document.visibilityState !== 'visible') {
        animationRef.current = null;
        return;
      }

      frameCountRef.current += 1;

      if (lastTimestampRef.current === null) {
        lastTimestampRef.current = timestamp;
      }

      processAnimationFrame(timestamp - lastTimestampRef.current);
      lastTimestampRef.current = timestamp;

      animationRef.current = requestAnimationFrame(animate);
    },
    [processAnimationFrame]
  );

  const stopAnimation = useCallback(() => {
    if (animationRef.current === null) {
      return;
    }

    cancelAnimationFrame(animationRef.current);
    animationRef.current = null;
  }, []);

  const startAnimation = useCallback(() => {
    if (animationRef.current !== null || document.visibilityState !== 'visible') {
      return;
    }

    animationRef.current = requestAnimationFrame(animate);
  }, [animate]);

  const handleMouseMove = useCallback((e: MouseEvent) => {
    mouseAreaRef.current.x = e.clientX;
    mouseAreaRef.current.y = e.clientY;
  }, []);

  const handleMouseOut = useCallback(() => {
    mouseAreaRef.current.x = null;
    mouseAreaRef.current.y = null;
  }, []);

  useEffect(() => {
    handleResize();

    window.addEventListener('resize', handleResize);
    window.addEventListener('mousemove', handleMouseMove);
    window.addEventListener('mouseout', handleMouseOut);

    const el = document.getElementById('auth-card-wrapper');
    let observer: ResizeObserver | null = null;
    if (el) {
      observer = new ResizeObserver(() => {
        handleResize();
      });
      observer.observe(el);
    }

    startAnimation();

    return () => {
      window.removeEventListener('resize', handleResize);
      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseout', handleMouseOut);
      if (observer) {
        observer.disconnect();
      }
      stopAnimation();
    };
  }, [handleResize, handleMouseMove, handleMouseOut, startAnimation, stopAnimation]);

  useEffect(() => {
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'hidden') {
        stopAnimation();
        return;
      }

      resetFrameTimingState();
      startAnimation();
    };

    document.addEventListener('visibilitychange', handleVisibilityChange);
    return () => document.removeEventListener('visibilitychange', handleVisibilityChange);
  }, [resetFrameTimingState, startAnimation, stopAnimation]);

  useEffect(() => {
    if (!shouldExposeAnimationDiagnostics()) {
      delete window.__AXONHUB_SIGNIN_ANIMATION__;
      return;
    }

    window.__AXONHUB_SIGNIN_ANIMATION__ = {
      reset: resetDiagnosticsState,
      snapshot: snapshotDiagnostics,
      simulate: simulateDiagnostics,
      simulateLargeGap: simulateLargeGapDiagnostics,
    };

    return () => {
      delete window.__AXONHUB_SIGNIN_ANIMATION__;
    };
  }, [resetDiagnosticsState, simulateDiagnostics, simulateLargeGapDiagnostics, snapshotDiagnostics]);

  return (
    <canvas ref={canvasRef} data-testid='sign-in-animation-canvas' className='pointer-events-none fixed inset-0' style={{ zIndex: 0 }} />
  );
};

export default AnimatedLineBackground;
