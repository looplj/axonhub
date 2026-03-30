export interface Particle {
  x: number;
  y: number;
  xa: number;
  ya: number;
  max: number;
}

export interface MouseArea {
  x: number | null;
  y: number | null;
  max: number;
}

export interface AnimationConfig {
  targetFps: number;
  frameIntervalMs: number;
  maxCatchUpMs: number;
  maxStepsPerFrame: number;
}

export const animationConfig: AnimationConfig = {
  targetFps: 60,
  frameIntervalMs: 1000 / 60,
  maxCatchUpMs: 100,
  maxStepsPerFrame: 5,
};

export interface FormBounds {
  formCenterX: number;
  formCenterY: number;
  formLeft: number;
  formRight: number;
  formTop: number;
  formBottom: number;
}

const LEFT_PARTICLE_FILL_STYLE = 'rgba(148, 163, 184, 0.4)';
const RIGHT_PARTICLE_FILL_STYLE = 'rgba(100, 116, 139, 0.3)';

function getLineIntersectsForm(startX: number, startY: number, endX: number, endY: number, bounds: FormBounds): boolean {
  return (
    (startX < bounds.formLeft && endX > bounds.formRight) ||
    (startX > bounds.formRight && endX < bounds.formLeft) ||
    (startY < bounds.formTop && endY > bounds.formBottom) ||
    (startY > bounds.formBottom && endY < bounds.formTop) ||
    isInFormArea(startX, startY, bounds) ||
    isInFormArea(endX, endY, bounds)
  );
}

function drawConnection(
  ctx: CanvasRenderingContext2D,
  canvasWidth: number,
  bounds: FormBounds,
  dot: Particle,
  targetX: number,
  targetY: number,
  targetMax: number
): void {
  const xc = dot.x - targetX;
  const yc = dot.y - targetY;
  const dis = xc * xc + yc * yc;

  if (dis >= targetMax || getLineIntersectsForm(dot.x, dot.y, targetX, targetY, bounds)) {
    return;
  }

  const ratio = (targetMax - dis) / targetMax;
  const avgX = (dot.x + targetX) / 2;
  const isLeftSide = avgX < canvasWidth / 2;
  const lineColor = isLeftSide ? `rgba(148, 163, 184, ${ratio * 0.4 + 0.1})` : `rgba(100, 116, 139, ${ratio * 0.3 + 0.1})`;

  ctx.beginPath();
  ctx.lineWidth = ratio / 2 + 0.5;
  ctx.strokeStyle = lineColor;
  ctx.moveTo(dot.x, dot.y);
  ctx.lineTo(targetX, targetY);
  ctx.stroke();
}

export function getFormBounds(canvasWidth: number, canvasHeight: number): FormBounds {
  const rightSideStart = canvasWidth / 2;
  const formCenterX = rightSideStart + canvasWidth / 2 / 2;
  const formCenterY = canvasHeight / 2;
  const formWidth = 360;
  const formHeight = 500;

  return {
    formCenterX,
    formCenterY,
    formLeft: formCenterX - formWidth / 2,
    formRight: formCenterX + formWidth / 2,
    formTop: formCenterY - formHeight / 2,
    formBottom: formCenterY + formHeight / 2,
  };
}

export function isInFormArea(x: number, y: number, bounds: FormBounds): boolean {
  return x >= bounds.formLeft && x <= bounds.formRight && y >= bounds.formTop && y <= bounds.formBottom;
}

export function initParticles(canvasWidth: number, canvasHeight: number, bounds: FormBounds = getFormBounds(canvasWidth, canvasHeight)): Particle[] {
  const particles: Particle[] = [];
  const particleCount = 120;
  const leftSideCount = Math.floor(particleCount * 0.6);
  const rightSideCount = particleCount - leftSideCount;

  for (let i = 0; i < leftSideCount; i++) {
    const x = Math.random() * (canvasWidth / 2 - 20);
    const y = Math.random() * canvasHeight;
    const xa = (Math.random() * 1 - 0.5) * 0.6;
    const ya = (Math.random() * 1 - 0.5) * 0.6;

    particles.push({
      x,
      y,
      xa,
      ya,
      max: 7000,
    });
  }

  for (let i = 0; i < rightSideCount; i++) {
    let x = 0;
    let y = 0;
    let attempts = 0;

    do {
      x = canvasWidth / 2 + 20 + Math.random() * (canvasWidth / 2 - 20);
      y = Math.random() * canvasHeight;
      attempts++;
    } while (isInFormArea(x, y, bounds) && attempts < 30);

    if (isInFormArea(x, y, bounds)) {
      if (Math.random() > 0.5) {
        x =
          Math.random() > 0.5
            ? canvasWidth / 2 + 20 + Math.random() * (bounds.formLeft - canvasWidth / 2 - 20)
            : bounds.formRight + Math.random() * (canvasWidth - bounds.formRight - 20);
      } else {
        y = Math.random() > 0.5 ? Math.random() * bounds.formTop : bounds.formBottom + Math.random() * (canvasHeight - bounds.formBottom);
      }
    }

    const xa = (Math.random() * 1 - 0.5) * 0.5;
    const ya = (Math.random() * 1 - 0.5) * 0.5;

    particles.push({
      x,
      y,
      xa,
      ya,
      max: 5000,
    });
  }

  return particles;
}

export function updateParticles(
  canvasWidth: number,
  canvasHeight: number,
  particles: Particle[],
  mouseArea: MouseArea,
  bounds: FormBounds = getFormBounds(canvasWidth, canvasHeight)
): void {
  for (let index = 0; index < particles.length; index += 1) {
    const dot = particles[index];
    dot.x += dot.xa;
    dot.y += dot.ya;

    dot.xa *= dot.x > canvasWidth || dot.x < 0 ? -1 : 1;
    dot.ya *= dot.y > canvasHeight || dot.y < 0 ? -1 : 1;

    if (isInFormArea(dot.x, dot.y, bounds)) {
      const pushForce = 0.5;
      if (dot.x < bounds.formCenterX) {
        dot.xa -= pushForce;
      } else {
        dot.xa += pushForce;
      }
      if (dot.y < bounds.formCenterY) {
        dot.ya -= pushForce;
      } else {
        dot.ya += pushForce;
      }
    }

    if (mouseArea.x !== null && mouseArea.y !== null) {
      const xc = dot.x - mouseArea.x;
      const yc = dot.y - mouseArea.y;
      const dis = xc * xc + yc * yc;

      if (dis < mouseArea.max && dis > mouseArea.max / 2) {
        dot.x -= xc * 0.015;
        dot.y -= yc * 0.015;
      }
    }
  }
}

export function renderParticles(
  ctx: CanvasRenderingContext2D,
  canvasWidth: number,
  canvasHeight: number,
  particles: Particle[],
  mouseArea: MouseArea,
  bounds: FormBounds = getFormBounds(canvasWidth, canvasHeight)
): void {
  ctx.clearRect(0, 0, canvasWidth, canvasHeight);

  let hasLeftParticles = false;
  ctx.beginPath();
  for (let index = 0; index < particles.length; index += 1) {
    const dot = particles[index];
    if (!isInFormArea(dot.x, dot.y, bounds) && dot.x < canvasWidth / 2) {
      ctx.rect(dot.x - 1.5, dot.y - 1.5, 3, 3);
      hasLeftParticles = true;
    }
  }
  if (hasLeftParticles) {
    ctx.fillStyle = LEFT_PARTICLE_FILL_STYLE;
    ctx.fill();
  }

  let hasRightParticles = false;
  ctx.beginPath();
  for (let index = 0; index < particles.length; index += 1) {
    const dot = particles[index];
    if (!isInFormArea(dot.x, dot.y, bounds) && dot.x >= canvasWidth / 2) {
      ctx.rect(dot.x - 1.5, dot.y - 1.5, 3, 3);
      hasRightParticles = true;
    }
  }
  if (hasRightParticles) {
    ctx.fillStyle = RIGHT_PARTICLE_FILL_STYLE;
    ctx.fill();
  }

  const hasMouseArea = mouseArea.x !== null && mouseArea.y !== null;
  for (let index = 0; index < particles.length; index += 1) {
    const dot = particles[index];

    if (hasMouseArea) {
      drawConnection(ctx, canvasWidth, bounds, dot, mouseArea.x, mouseArea.y, mouseArea.max);
    }

    for (let nextIndex = index + 1; nextIndex < particles.length; nextIndex += 1) {
      const nextDot = particles[nextIndex];
      drawConnection(ctx, canvasWidth, bounds, dot, nextDot.x, nextDot.y, nextDot.max);
    }
  }
}

export function stepAnimation(
  ctx: CanvasRenderingContext2D,
  canvasWidth: number,
  canvasHeight: number,
  particles: Particle[],
  mouseArea: MouseArea,
  bounds: FormBounds = getFormBounds(canvasWidth, canvasHeight)
): void {
  updateParticles(canvasWidth, canvasHeight, particles, mouseArea, bounds);
  renderParticles(ctx, canvasWidth, canvasHeight, particles, mouseArea, bounds);
}
