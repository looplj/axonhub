import {
  Circle,
  MessageSquare,
  Sparkles,
  Wrench,
  CheckCircle2,
  Image,
  Settings,
  type LucideIcon,
} from 'lucide-react'

/**
 * 根据 span 类型返回对应的图标组件
 * Returns the appropriate icon component based on span type
 */
export function getSpanIcon(spanType: string): LucideIcon {
  const normalizedType = spanType?.toLowerCase() || ''

  switch (normalizedType) {
    case 'user_query':
    case 'text':
    case 'message':
      return MessageSquare
    case 'thinking':
    case 'llm':
      return Sparkles
    case 'tool_use':
    case 'function_call':
      return Wrench
    case 'tool_result':
    case 'function_result':
      return CheckCircle2
    case 'user_image_url':
    case 'image_url':
      return Image
    case 'system_instruction':
      return Settings
    default:
      return Circle
  }
}
