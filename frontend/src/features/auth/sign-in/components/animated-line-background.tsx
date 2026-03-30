import { type FC, useCallback, useEffect, useRef } from 'react';
import {
  type MouseArea,
  type Particle,
  animationConfig,
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
  const animationRef = useRef<number>(null);
  const particlesRef = useRef<Particle[]>([]);
  const diagnosticsInitialParticlesRef = useRef<Particle[] | null>(null);
  const diagnosticsCanvasSizeRef = useRef<{ width: number; height: number } | null>(null);
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

  const renderFrame = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    renderParticles(ctx, canvas.width, canvas.height, particlesRef.current, mouseAreaRef.current);

    renderCountRef.current += 1;
  }, []);

  const applyAnimationStep = useCallback((deltaMs: number) => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    updateParticles(canvas.width, canvas.height, particlesRef.current, mouseAreaRef.current);

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

      renderFrame();
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
    accumulatorRef.current = 0;
    lastTimestampRef.current = null;
    lastFrameDeltaMsRef.current = 0;
    lastClampedDeltaMsRef.current = 0;
    lastFrameStepCountRef.current = 0;
    lastAppliedDeltaMsRef.current = 0;
    mouseAreaRef.current = createMouseArea();

    resize();

    const nextCanvasSize = { width: canvas.width, height: canvas.height };
    const needsNewInitialParticles =
      diagnosticsInitialParticlesRef.current === null ||
      diagnosticsCanvasSizeRef.current?.width !== nextCanvasSize.width ||
      diagnosticsCanvasSizeRef.current?.height !== nextCanvasSize.height;

    if (needsNewInitialParticles) {
      diagnosticsInitialParticlesRef.current = initParticles(nextCanvasSize.width, nextCanvasSize.height);
      diagnosticsCanvasSizeRef.current = nextCanvasSize;
    }

    particlesRef.current = cloneParticles(diagnosticsInitialParticlesRef.current);
    renderFrame();
    renderCountRef.current = 0;
  }, [renderFrame, resize]);

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
      animationRef.current = requestAnimationFrame(animate);
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

  return (
    <canvas ref={canvasRef} data-testid='sign-in-animation-canvas' className='pointer-events-none fixed inset-0' style={{ zIndex: 0 }} />
  );
};

export default AnimatedLineBackground;
