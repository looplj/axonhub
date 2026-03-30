import { useCallback, useEffect, useRef, type FC } from 'react';

import { type MouseArea, type Particle, initParticles, stepAnimation } from './animated-line-background.engine';

interface AnimationDiagnosticsSnapshot {
  targetFps: number;
  frameIntervalMs: number;
  frameCount: number;
  renderCount: number;
  simulatedMs: number;
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

const TARGET_FPS = 60;
const FRAME_INTERVAL_MS = 1000 / TARGET_FPS;
const DEBUG_QUERY_PARAM = '__axonhub_debug_animation';

const createMouseArea = (): MouseArea => ({ x: null, y: null, max: 20000 });

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
  const animationRef = useRef<number>(null);
  const particlesRef = useRef<Particle[]>([]);
  const mouseAreaRef = useRef<MouseArea>(createMouseArea());
  const frameCountRef = useRef(0);
  const renderCountRef = useRef(0);
  const simulatedMsRef = useRef(0);
  const lastAppliedDeltaMsRef = useRef(0);

  const resize = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    canvas.width = window.innerWidth;
    canvas.height = window.innerHeight;
  }, []);

  const initializeParticles = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    particlesRef.current = initParticles(canvas.width, canvas.height);
  }, []);

  const applyAnimationStep = useCallback((deltaMs: number) => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    stepAnimation(ctx, canvas.width, canvas.height, particlesRef.current, mouseAreaRef.current);

    renderCountRef.current += 1;
    simulatedMsRef.current += deltaMs;
    lastAppliedDeltaMsRef.current = deltaMs;
  }, []);

  const resetDiagnosticsState = useCallback(() => {
    frameCountRef.current = 0;
    renderCountRef.current = 0;
    simulatedMsRef.current = 0;
    lastAppliedDeltaMsRef.current = 0;
    mouseAreaRef.current = createMouseArea();

    resize();
    initializeParticles();
  }, [initializeParticles, resize]);

  const snapshotDiagnostics = useCallback<() => AnimationDiagnosticsSnapshot>(() => {
    return {
      targetFps: TARGET_FPS,
      frameIntervalMs: FRAME_INTERVAL_MS,
      frameCount: frameCountRef.current,
      renderCount: renderCountRef.current,
      simulatedMs: simulatedMsRef.current,
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
        applyAnimationStep(safeStepMs);
      }
    },
    [applyAnimationStep],
  );

  const simulateLargeGapDiagnostics = useCallback(
    (deltaMs: number) => {
      const safeDeltaMs = Number.isFinite(deltaMs) ? Math.max(0, deltaMs) : 0;

      frameCountRef.current += 1;
      applyAnimationStep(safeDeltaMs);
    },
    [applyAnimationStep],
  );

  const animate = useCallback(() => {
    frameCountRef.current += 1;
    applyAnimationStep(FRAME_INTERVAL_MS);

    animationRef.current = requestAnimationFrame(animate);
  }, [applyAnimationStep]);

  const handleMouseMove = useCallback((e: MouseEvent) => {
    mouseAreaRef.current.x = e.clientX;
    mouseAreaRef.current.y = e.clientY;
  }, []);

  const handleMouseOut = useCallback(() => {
    mouseAreaRef.current.x = null;
    mouseAreaRef.current.y = null;
  }, []);

  useEffect(() => {
    resize();
    // 强制重新初始化粒子
    const initTimer = setTimeout(() => {
      initializeParticles();
    }, 50);

    window.addEventListener('resize', resize);
    window.addEventListener('mousemove', handleMouseMove);
    window.addEventListener('mouseout', handleMouseOut);

    // 延迟200ms开始动画，确保粒子初始化完成
    const timer = setTimeout(() => {
      animate();
    }, 200);

    return () => {
      window.removeEventListener('resize', resize);
      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseout', handleMouseOut);
      if (animationRef.current) {
        cancelAnimationFrame(animationRef.current);
      }
      clearTimeout(initTimer);
      clearTimeout(timer);
    };
  }, [resize, initializeParticles, animate, handleMouseMove, handleMouseOut]);

  useEffect(() => {
    const handleResize = () => {
      resize();
      initializeParticles();
    };

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [resize, initializeParticles]);

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

  return <canvas ref={canvasRef} data-testid='sign-in-animation-canvas' className='pointer-events-none fixed inset-0' style={{ zIndex: 0 }} />;
};

export default AnimatedLineBackground;
