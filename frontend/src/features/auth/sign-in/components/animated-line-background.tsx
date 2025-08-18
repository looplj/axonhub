import React, { useEffect, useRef, useCallback } from 'react'

interface Particle {
  x: number
  y: number
  xa: number
  ya: number
  max: number
}

interface MouseArea {
  x: number | null
  y: number | null
  max: number
}

const AnimatedLineBackground: React.FC = () => {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const animationRef = useRef<number>()
  const particlesRef = useRef<Particle[]>([])
  const mouseAreaRef = useRef<MouseArea>({ x: null, y: null, max: 20000 })

  const resize = useCallback(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    canvas.width = window.innerWidth
    canvas.height = window.innerHeight
  }, [canvasRef])

  const initParticles = useCallback(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    particlesRef.current = []
    const particleCount = 150
    
    for (let i = 0; i < particleCount; i++) {
      const x = Math.random() * canvas.width
      const y = Math.random() * canvas.height
      const xa = (Math.random() * 1 - 0.5) * 0.8 // 进一步降低速度
      const ya = (Math.random() * 1 - 0.5) * 0.8
      
      particlesRef.current.push({
        x,
        y,
        xa,
        ya,
        max: 6000
      })
    }
  }, [canvasRef])

  const animate = useCallback(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    const ctx = canvas.getContext('2d')
    if (!ctx) return

    ctx.clearRect(0, 0, canvas.width, canvas.height)

    ctx.fillStyle = 'rgba(84, 61, 50, 0.8)' 

    const ndots = [mouseAreaRef.current, ...particlesRef.current]

    particlesRef.current.forEach((dot) => {
      // 粒子位移
      dot.x += dot.xa
      dot.y += dot.ya

      // 遇到边界将加速度反向
      dot.xa *= (dot.x > canvas.width || dot.x < 0) ? -1 : 1
      dot.ya *= (dot.y > canvas.height || dot.y < 0) ? -1 : 1

      // 绘制点
      ctx.fillRect(dot.x - 1.5, dot.y - 1.5, 3, 3)

      // 循环比对粒子间的距离
      for (let i = 0; i < ndots.length; i++) {
        const d2 = ndots[i]
        if (dot === d2 || d2.x === null || d2.y === null) continue

        const xc = dot.x - d2.x
        const yc = dot.y - d2.y
        const dis = xc * xc + yc * yc

        if (dis < d2.max) {
          // 如果是鼠标，则让粒子向鼠标的位置移动
          if (d2 === mouseAreaRef.current && dis > (d2.max / 2)) {
            dot.x -= xc * 0.015 // 降低鼠标吸引力
            dot.y -= yc * 0.015
          }

          // 计算距离比
          const ratio = (d2.max - dis) / d2.max

          // 画线 - 使用橙色线条
          ctx.beginPath()
          ctx.lineWidth = ratio / 2 + 1 // 稍微增加线条宽度
          ctx.strokeStyle = `rgba(84, 61, 50, ${ratio * 0.6 + 0.2})` // 橙色，更高透明度
          ctx.moveTo(dot.x, dot.y)
          ctx.lineTo(d2.x, d2.y)
          ctx.stroke()
        }
      }

      // 将已经计算过的粒子从数组中删除
      ndots.splice(ndots.indexOf(dot), 1)
    })

    animationRef.current = requestAnimationFrame(animate)
  }, [])

  const handleMouseMove = useCallback((e: MouseEvent) => {
    mouseAreaRef.current.x = e.clientX
    mouseAreaRef.current.y = e.clientY
  }, [])

  const handleMouseOut = useCallback(() => {
    mouseAreaRef.current.x = null
    mouseAreaRef.current.y = null
  }, [])

  useEffect(() => {
    resize()
    initParticles()

    window.addEventListener('resize', resize)
    window.addEventListener('mousemove', handleMouseMove)
    window.addEventListener('mouseout', handleMouseOut)

    // 延迟100ms开始动画
    const timer = setTimeout(() => {
      animate()
    }, 100)

    return () => {
      window.removeEventListener('resize', resize)
      window.removeEventListener('mousemove', handleMouseMove)
      window.removeEventListener('mouseout', handleMouseOut)
      if (animationRef.current) {
        cancelAnimationFrame(animationRef.current)
      }
      clearTimeout(timer)
    }
  }, [resize, initParticles, animate, handleMouseMove, handleMouseOut])

  useEffect(() => {
    const handleResize = () => {
      resize()
      initParticles()
    }

    window.addEventListener('resize', handleResize)
    return () => window.removeEventListener('resize', handleResize)
  }, [resize, initParticles])

  return (
    <canvas
      ref={canvasRef}
      className="fixed inset-0 pointer-events-none"
      style={{ zIndex: 1 }}
    />
  )
}

export default AnimatedLineBackground
