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

interface FormBounds {
  formCenterX: number;
  formCenterY: number;
  formLeft: number;
  formRight: number;
  formTop: number;
  formBottom: number;
}

type DrawNode = Particle | MouseArea;

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

export function initParticles(canvasWidth: number, canvasHeight: number): Particle[] {
  const particles: Particle[] = [];
  const particleCount = 120;
  const bounds = getFormBounds(canvasWidth, canvasHeight);
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

export function stepAnimation(
  ctx: CanvasRenderingContext2D,
  canvasWidth: number,
  canvasHeight: number,
  particles: Particle[],
  mouseArea: MouseArea,
): void {
  const bounds = getFormBounds(canvasWidth, canvasHeight);

  ctx.clearRect(0, 0, canvasWidth, canvasHeight);

  const ndots: DrawNode[] = [mouseArea, ...particles];

  particles.forEach((dot) => {
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

    if (!isInFormArea(dot.x, dot.y, bounds)) {
      const isLeftSide = dot.x < canvasWidth / 2;
      ctx.fillStyle = isLeftSide ? 'rgba(148, 163, 184, 0.4)' : 'rgba(100, 116, 139, 0.3)';
      ctx.fillRect(dot.x - 1.5, dot.y - 1.5, 3, 3);
    }

    for (let i = 0; i < ndots.length; i++) {
      const d2 = ndots[i];
      if (dot === d2 || d2.x === null || d2.y === null) continue;

      const xc = dot.x - d2.x;
      const yc = dot.y - d2.y;
      const dis = xc * xc + yc * yc;

      if (dis < d2.max) {
        if (d2 === mouseArea && dis > d2.max / 2) {
          dot.x -= xc * 0.015;
          dot.y -= yc * 0.015;
        }

        const ratio = (d2.max - dis) / d2.max;
        const lineIntersectsForm =
          (dot.x < bounds.formLeft && d2.x > bounds.formRight) ||
          (dot.x > bounds.formRight && d2.x < bounds.formLeft) ||
          (dot.y < bounds.formTop && d2.y > bounds.formBottom) ||
          (dot.y > bounds.formBottom && d2.y < bounds.formTop) ||
          isInFormArea(dot.x, dot.y, bounds) ||
          isInFormArea(d2.x, d2.y, bounds);

        if (!lineIntersectsForm) {
          ctx.beginPath();
          ctx.lineWidth = ratio / 2 + 0.5;

          const avgX = (dot.x + d2.x) / 2;
          const isLeftSide = avgX < canvasWidth / 2;
          const lineColor = isLeftSide ? `rgba(148, 163, 184, ${ratio * 0.4 + 0.1})` : `rgba(100, 116, 139, ${ratio * 0.3 + 0.1})`;

          ctx.strokeStyle = lineColor;
          ctx.moveTo(dot.x, dot.y);
          ctx.lineTo(d2.x, d2.y);
          ctx.stroke();
        }
      }
    }

    ndots.splice(ndots.indexOf(dot), 1);
  });
}
