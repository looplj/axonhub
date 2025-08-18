import React, { useEffect, useState } from 'react'

interface Point {
  x: number
  y: number
}

interface Line {
  start: Point
  end: Point
  id: string
  opacity: number
  animationDelay: number
}

const GeometricBackground: React.FC = () => {
  const [lines, setLines] = useState<Line[]>([])
  const [dimensions, setDimensions] = useState({ width: 0, height: 0 })

  useEffect(() => {
    const updateDimensions = () => {
      setDimensions({
        width: window.innerWidth,
        height: window.innerHeight
      })
    }

    updateDimensions()
    window.addEventListener('resize', updateDimensions)

    return () => window.removeEventListener('resize', updateDimensions)
  }, [])

  useEffect(() => {
    if (dimensions.width === 0 || dimensions.height === 0) return

    const generateRandomLines = () => {
      const newLines: Line[] = []
      const numberOfLines = Math.min(150, Math.floor((dimensions.width * dimensions.height) / 12000)) // Optimized density

      // Create grid-based points for more structured look
      const gridSize = 100
      const gridPoints: Point[] = []
      
      for (let x = 0; x < dimensions.width; x += gridSize) {
        for (let y = 0; y < dimensions.height; y += gridSize) {
          // Add some randomness to grid positions
          gridPoints.push({
            x: x + (Math.random() - 0.5) * 50,
            y: y + (Math.random() - 0.5) * 50
          })
        }
      }

      for (let i = 0; i < numberOfLines; i++) {
        let start: Point
        let end: Point

        if (Math.random() > 0.3 && gridPoints.length > 1) {
          // Use grid-based connections for more structured look
          const startIndex = Math.floor(Math.random() * gridPoints.length)
          const endIndex = Math.floor(Math.random() * gridPoints.length)
          start = gridPoints[startIndex]
          end = gridPoints[endIndex]
          
          // Ensure minimum distance between points
          const distance = Math.sqrt(Math.pow(end.x - start.x, 2) + Math.pow(end.y - start.y, 2))
          if (distance < 50 || distance > 300) continue
        } else {
          // Generate completely random lines
          start = {
            x: Math.random() * dimensions.width,
            y: Math.random() * dimensions.height
          }

          const angle = Math.random() * Math.PI * 2
          const length = 80 + Math.random() * 180
          
          end = {
            x: start.x + Math.cos(angle) * length,
            y: start.y + Math.sin(angle) * length
          }
        }

        // Ensure lines stay within bounds with margin
        if (end.x >= 20 && end.x <= dimensions.width - 20 && 
            end.y >= 20 && end.y <= dimensions.height - 20 &&
            start.x >= 20 && start.x <= dimensions.width - 20 &&
            start.y >= 20 && start.y <= dimensions.height - 20) {
          newLines.push({
            start,
            end,
            id: `line-${i}`,
            opacity: 0.15 + Math.random() * 0.25, // Slightly higher opacity
            animationDelay: Math.random() * 8 // Reduced delay range
          })
        }
      }

      setLines(newLines)
    }

    generateRandomLines()
  }, [dimensions])

  return (
    <div className="fixed inset-0 pointer-events-none overflow-hidden">
      <svg
        width={dimensions.width}
        height={dimensions.height}
        className="absolute inset-0"
        style={{ zIndex: 1 }}
      >
        <defs>
          <linearGradient id="lineGradient" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stopColor="rgb(251, 146, 60)" stopOpacity="0.8" />
            <stop offset="50%" stopColor="rgb(249, 115, 22)" stopOpacity="0.6" />
            <stop offset="100%" stopColor="rgb(234, 88, 12)" stopOpacity="0.4" />
          </linearGradient>
        </defs>
        
        {lines.map((line) => (
          <g key={line.id}>
            {/* Main line */}
            <line
              x1={line.start.x}
              y1={line.start.y}
              x2={line.end.x}
              y2={line.end.y}
              stroke="url(#lineGradient)"
              strokeWidth="1"
              opacity={line.opacity}
              className="animate-geometric-pulse"
              style={{
                animationDelay: `${line.animationDelay}s`,
                filter: 'drop-shadow(0 0 2px rgba(251, 146, 60, 0.3))'
              }}
            />
            
            {/* Connection points */}
            <circle
              cx={line.start.x}
              cy={line.start.y}
              r="2"
              fill="rgb(251, 146, 60)"
              opacity={line.opacity * 1.5}
              className="animate-geometric-glow"
              style={{
                animationDelay: `${line.animationDelay}s`,
                filter: 'drop-shadow(0 0 4px rgba(251, 146, 60, 0.5))'
              }}
            />
            
            <circle
              cx={line.end.x}
              cy={line.end.y}
              r="2"
              fill="rgb(249, 115, 22)"
              opacity={line.opacity * 1.5}
              className="animate-geometric-glow"
              style={{
                animationDelay: `${line.animationDelay + 0.5}s`,
                filter: 'drop-shadow(0 0 4px rgba(249, 115, 22, 0.5))'
              }}
            />
          </g>
        ))}
        
        {/* Additional network connections */}
        {lines.slice(0, Math.floor(lines.length / 3)).map((line, index) => {
          const nextLine = lines[index + 1]
          if (!nextLine) return null
          
          return (
            <line
              key={`connection-${line.id}`}
              x1={line.end.x}
              y1={line.end.y}
              x2={nextLine.start.x}
              y2={nextLine.start.y}
              stroke="rgb(234, 88, 12)"
              strokeWidth="0.5"
              opacity={line.opacity * 0.5}
              className="animate-geometric-fade"
              style={{
                animationDelay: `${line.animationDelay + 2}s`,
                strokeDasharray: '5,5'
              }}
            />
          )
        })}
      </svg>
    </div>
  )
}

export default GeometricBackground
