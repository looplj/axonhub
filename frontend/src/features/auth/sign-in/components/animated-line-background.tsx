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
    const particleCount = 120 // Reduced for better performance with split layout
    
    // 定义表单区域 (与animate函数中的定义保持一致)
    const rightSideStart = canvas.width / 2
    const formCenterX = rightSideStart + (canvas.width / 2) / 2 // 右侧区域的中心
    const formCenterY = canvas.height / 2
    const formWidth = 360
    const formHeight = 500
    const formLeft = formCenterX - formWidth / 2
    const formRight = formCenterX + formWidth / 2
    const formTop = formCenterY - formHeight / 2
    const formBottom = formCenterY + formHeight / 2
    
    const isInFormArea = (x: number, y: number) => {
      return x >= formLeft && x <= formRight && y >= formTop && y <= formBottom
    }
    
    // 分别为左右两侧生成粒子
    const leftSideCount = Math.floor(particleCount * 0.6) // 左侧更多粒子
    const rightSideCount = particleCount - leftSideCount
    
    // 左侧粒子 (品牌区域)
    for (let i = 0; i < leftSideCount; i++) {
      const x = Math.random() * (canvas.width / 2 - 20) // 避免太靠近中线
      const y = Math.random() * canvas.height
      const xa = (Math.random() * 1 - 0.5) * 0.6
      const ya = (Math.random() * 1 - 0.5) * 0.6
      
      particlesRef.current.push({
        x,
        y,
        xa,
        ya,
        max: 7000
      })
    }
    
    // 右侧粒子 (表单区域) - 避开表单区域
    for (let i = 0; i < rightSideCount; i++) {
      let x, y
      let attempts = 0
      
      do {
        x = (canvas.width / 2 + 20) + Math.random() * (canvas.width / 2 - 20)
        y = Math.random() * canvas.height
        attempts++
      } while (isInFormArea(x, y) && attempts < 30)
      
      // 如果仍在表单区域，放置在表单区域外
      if (isInFormArea(x, y)) {
        if (Math.random() > 0.5) {
          x = Math.random() > 0.5 ? 
            (canvas.width / 2 + 20) + Math.random() * (formLeft - canvas.width / 2 - 20) : 
            formRight + Math.random() * (canvas.width - formRight - 20)
        } else {
          y = Math.random() > 0.5 ? 
            Math.random() * formTop : 
            formBottom + Math.random() * (canvas.height - formBottom)
        }
      }
      
      const xa = (Math.random() * 1 - 0.5) * 0.5
      const ya = (Math.random() * 1 - 0.5) * 0.5
      
      particlesRef.current.push({
        x,
        y,
        xa,
        ya,
        max: 5000
      })
    }
  }, [canvasRef])

  const animate = useCallback(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    const ctx = canvas.getContext('2d')
    if (!ctx) return

    ctx.clearRect(0, 0, canvas.width, canvas.height)

    // 定义登录表单区域 (右侧区域，适应新的左右布局)
    const rightSideStart = canvas.width / 2
    const formCenterX = rightSideStart + (canvas.width / 2) / 2 // 右侧区域的中心
    const formCenterY = canvas.height / 2
    const formWidth = 360
    const formHeight = 500
    const formLeft = formCenterX - formWidth / 2
    const formRight = formCenterX + formWidth / 2
    const formTop = formCenterY - formHeight / 2
    const formBottom = formCenterY + formHeight / 2

    // 检查点是否在表单区域内
    const isInFormArea = (x: number, y: number) => {
      return x >= formLeft && x <= formRight && y >= formTop && y <= formBottom
    }

    const ndots = [mouseAreaRef.current, ...particlesRef.current]

    particlesRef.current.forEach((dot) => {
      // 粒子位移
      dot.x += dot.xa
      dot.y += dot.ya

      // 遇到边界将加速度反向
      dot.xa *= (dot.x > canvas.width || dot.x < 0) ? -1 : 1
      dot.ya *= (dot.y > canvas.height || dot.y < 0) ? -1 : 1

      // 如果粒子进入表单区域，推开它们
      if (isInFormArea(dot.x, dot.y)) {
        const pushForce = 0.5
        if (dot.x < formCenterX) {
          dot.xa -= pushForce
        } else {
          dot.xa += pushForce
        }
        if (dot.y < formCenterY) {
          dot.ya -= pushForce
        } else {
          dot.ya += pushForce
        }
      }

      // 只在表单区域外绘制点，使用不同颜色
      if (!isInFormArea(dot.x, dot.y)) {
        // Use different colors for left and right sides
        const isLeftSide = dot.x < canvas.width / 2
        ctx.fillStyle = isLeftSide ? 'rgba(148, 163, 184, 0.4)' : 'rgba(100, 116, 139, 0.3)'
        ctx.fillRect(dot.x - 1.5, dot.y - 1.5, 3, 3)
      }

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

          // 检查连线是否穿过表单区域，如果是则不绘制
          const lineIntersectsForm = (
            (dot.x < formLeft && d2.x > formRight) ||
            (dot.x > formRight && d2.x < formLeft) ||
            (dot.y < formTop && d2.y > formBottom) ||
            (dot.y > formBottom && d2.y < formTop) ||
            isInFormArea(dot.x, dot.y) ||
            isInFormArea(d2.x, d2.y)
          )

          // 只在不穿过表单区域时画线
          if (!lineIntersectsForm) {
            ctx.beginPath()
            ctx.lineWidth = ratio / 2 + 0.5
            // Use elegant colors for lines based on position
            const avgX = (dot.x + d2.x) / 2
            const isLeftSide = avgX < canvas.width / 2
            const lineColor = isLeftSide ? 
              `rgba(148, 163, 184, ${ratio * 0.4 + 0.1})` : 
              `rgba(100, 116, 139, ${ratio * 0.3 + 0.1})`
            ctx.strokeStyle = lineColor
            ctx.moveTo(dot.x, dot.y)
            ctx.lineTo(d2.x, d2.y)
            ctx.stroke()
          }
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
    // 强制重新初始化粒子
    setTimeout(() => {
      initParticles()
    }, 50)

    window.addEventListener('resize', resize)
    window.addEventListener('mousemove', handleMouseMove)
    window.addEventListener('mouseout', handleMouseOut)

    // 延迟200ms开始动画，确保粒子初始化完成
    const timer = setTimeout(() => {
      animate()
    }, 200)

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
      style={{ zIndex: 0 }}
    />
  )
}

export default AnimatedLineBackground
