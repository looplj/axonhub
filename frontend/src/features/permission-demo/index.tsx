import { useState, useEffect } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { useAuthStore } from '@/stores/authStore'
import { useRoutePermissions } from '@/hooks/useRoutePermissions'
import { routeConfigs } from '@/config/route-permission'
import { IconShield, IconEye, IconEyeOff, IconLock, IconLockOpen, IconRefresh } from '@tabler/icons-react'

// 演示用的本地存储键
const DEMO_USER_KEY = 'axonhub_demo_user'
const DEMO_ORIGINAL_USER_KEY = 'axonhub_demo_original_user'

export default function PermissionDemo() {
  const { user, setUser } = useAuthStore((state) => state.auth)
  const { userScopes, isOwner, checkRouteAccess } = useRoutePermissions()
  const [demoScopes, setDemoScopes] = useState<string[]>([])
  const [isDemoMode, setIsDemoMode] = useState(false)
  const [originalUser, setOriginalUser] = useState<any>(null)

  // 所有可用的scopes
  const allScopes = [
    'read_dashboard', 'read_users', 'read_roles', 'read_channels',
    'read_requests', 'read_api_keys', 'read_settings', 'write_users',
    'write_roles', 'write_channels', 'write_requests', 'write_api_keys',
    'write_settings', 'admin'
  ]

  // 初始化演示模式
  useEffect(() => {
    const savedDemoUser = localStorage.getItem(DEMO_USER_KEY)
    const savedOriginalUser = localStorage.getItem(DEMO_ORIGINAL_USER_KEY)
    
    if (savedDemoUser && savedOriginalUser) {
      try {
        const demoUser = JSON.parse(savedDemoUser)
        const origUser = JSON.parse(savedOriginalUser)
        setOriginalUser(origUser)
        setDemoScopes(demoUser.scopes || [])
        setIsDemoMode(true)
        setUser(demoUser)
      } catch (error) {
        console.warn('Failed to restore demo state:', error)
        localStorage.removeItem(DEMO_USER_KEY)
        localStorage.removeItem(DEMO_ORIGINAL_USER_KEY)
      }
    } else if (user) {
      setDemoScopes(user.scopes || [])
    }
  }, [])

  // 进入演示模式
  const enterDemoMode = () => {
    if (user && !isDemoMode) {
      setOriginalUser(user)
      localStorage.setItem(DEMO_ORIGINAL_USER_KEY, JSON.stringify(user))
      setIsDemoMode(true)
      
      const demoUser = {
        ...user,
        scopes: demoScopes
      }
      setUser(demoUser)
      localStorage.setItem(DEMO_USER_KEY, JSON.stringify(demoUser))
    }
  }

  // 退出演示模式
  const exitDemoMode = () => {
    if (originalUser) {
      setUser(originalUser)
      setIsDemoMode(false)
      setOriginalUser(null)
      localStorage.removeItem(DEMO_USER_KEY)
      localStorage.removeItem(DEMO_ORIGINAL_USER_KEY)
      setDemoScopes(originalUser.scopes || [])
    }
  }

  const handleScopeToggle = (scope: string) => {
    const newScopes = demoScopes.includes(scope)
      ? demoScopes.filter(s => s !== scope)
      : [...demoScopes, scope]
    
    setDemoScopes(newScopes)
    
    if (isDemoMode && user) {
      const updatedUser = {
        ...user,
        scopes: newScopes
      }
      setUser(updatedUser)
      localStorage.setItem(DEMO_USER_KEY, JSON.stringify(updatedUser))
    }
  }

  const setOwnerMode = (isOwnerMode: boolean) => {
    if (!isDemoMode) {
      enterDemoMode()
    }
    
    if (user) {
      const updatedUser = {
        ...user,
        isOwner: isOwnerMode,
        scopes: isOwnerMode ? [] : demoScopes
      }
      setUser(updatedUser)
      localStorage.setItem(DEMO_USER_KEY, JSON.stringify(updatedUser))
    }
  }

  const resetToDefaults = () => {
    const defaultScopes = ['read_dashboard', 'read_requests']
    setDemoScopes(defaultScopes)
    
    if (isDemoMode && user) {
      const updatedUser = {
        ...user,
        isOwner: false,
        scopes: defaultScopes
      }
      setUser(updatedUser)
      localStorage.setItem(DEMO_USER_KEY, JSON.stringify(updatedUser))
    }
  }

  return (
    <div className="container mx-auto p-6 space-y-6">
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-2">
          <IconShield className="h-6 w-6" />
          <h1 className="text-2xl font-bold">动态路由权限演示</h1>
        </div>
        <div className="flex items-center gap-2">
          {isDemoMode && (
            <Badge variant="secondary" className="flex items-center gap-1">
              <IconRefresh className="h-3 w-3" />
              演示模式
            </Badge>
          )}
          <Button
            variant={isDemoMode ? "destructive" : "default"}
            size="sm"
            onClick={isDemoMode ? exitDemoMode : enterDemoMode}
          >
            {isDemoMode ? "退出演示" : "进入演示"}
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* 用户权限控制面板 */}
        <Card>
          <CardHeader>
            <CardTitle>权限控制面板</CardTitle>
            <CardDescription>
              切换不同的权限来查看路由访问控制的效果
              {!isDemoMode && (
                <span className="block text-orange-600 mt-1">
                  点击"进入演示"开始测试权限控制
                </span>
              )}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center justify-between">
              <Label>Owner 权限</Label>
              <div className="flex gap-2">
                <Button 
                  size="sm" 
                  variant={isOwner ? "default" : "outline"}
                  onClick={() => setOwnerMode(true)}
                  disabled={!isDemoMode}
                >
                  启用 Owner
                </Button>
                <Button 
                  size="sm" 
                  variant={!isOwner ? "default" : "outline"}
                  onClick={() => setOwnerMode(false)}
                  disabled={!isDemoMode}
                >
                  普通用户
                </Button>
              </div>
            </div>

            <Separator />

            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <Label className="text-sm font-medium">用户权限范围 (Scopes)</Label>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={resetToDefaults}
                  disabled={!isDemoMode || isOwner}
                >
                  重置默认
                </Button>
              </div>
              <div className="grid grid-cols-1 gap-2">
                {allScopes.map((scope) => (
                  <div key={scope} className="flex items-center justify-between">
                    <Label htmlFor={scope} className="text-sm">
                      {scope}
                    </Label>
                    <Switch
                      id={scope}
                      checked={demoScopes.includes(scope)}
                      onCheckedChange={() => handleScopeToggle(scope)}
                      disabled={!isDemoMode || isOwner}
                    />
                  </div>
                ))}
              </div>
            </div>

            <div className="pt-4">
              <Label className="text-sm font-medium">当前有效权限:</Label>
              <div className="flex flex-wrap gap-1 mt-2">
                {userScopes.map((scope) => (
                  <Badge key={scope} variant="secondary" className="text-xs">
                    {scope}
                  </Badge>
                ))}
                {isOwner && (
                  <Badge variant="default" className="text-xs">
                    Owner (全部权限)
                  </Badge>
                )}
              </div>
            </div>
          </CardContent>
        </Card>

        {/* 路由访问状态 */}
        <Card>
          <CardHeader>
            <CardTitle>路由访问状态</CardTitle>
            <CardDescription>
              查看当前权限下各路由的访问状态
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {routeConfigs.map((group) => (
                <div key={group.title} className="space-y-2">
                  <h4 className="font-medium text-sm text-muted-foreground">
                    {group.title}
                  </h4>
                  <div className="space-y-1">
                    {group.routes.map((route) => {
                      const access = checkRouteAccess(route.path)
                      return (
                        <div
                          key={route.path}
                          className="flex items-center justify-between p-2 rounded border"
                        >
                          <div className="flex items-center gap-2">
                            <span className="text-sm font-mono">
                              {route.path}
                            </span>
                            {route.requiredScopes && (
                              <Badge variant="outline" className="text-xs">
                                {route.requiredScopes.join(', ')}
                              </Badge>
                            )}
                          </div>
                          <div className="flex items-center gap-2">
                            {access.hasAccess ? (
                              <>
                                <IconLockOpen className="h-4 w-4 text-green-500" />
                                <Badge variant="default" className="text-xs">
                                  可访问
                                </Badge>
                              </>
                            ) : (
                              <>
                                {access.mode === 'hidden' ? (
                                  <IconEyeOff className="h-4 w-4 text-red-500" />
                                ) : (
                                  <IconLock className="h-4 w-4 text-orange-500" />
                                )}
                                <Badge 
                                  variant={access.mode === 'hidden' ? 'destructive' : 'secondary'} 
                                  className="text-xs"
                                >
                                  {access.mode === 'hidden' ? '隐藏' : '禁用'}
                                </Badge>
                              </>
                            )}
                          </div>
                        </div>
                      )
                    })}
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>说明</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm text-muted-foreground">
          <p>• <strong>演示模式</strong>: 点击"进入演示"开始测试，不会影响真实的用户权限</p>
          <p>• <strong>Owner 权限</strong>: 拥有所有权限，可以访问所有路由</p>
          <p>• <strong>隐藏模式</strong>: 没有权限时，路由在侧边栏中完全隐藏</p>
          <p>• <strong>禁用模式</strong>: 没有权限时，路由在侧边栏中显示但禁用</p>
          <p>• <strong>组级隐藏</strong>: 如果一个组的所有路由都不可访问，整个组会被隐藏</p>
          <p>• <strong>权限持久化</strong>: 演示模式下的权限设置会保存到本地存储，刷新页面后保持不变</p>
          <p>• 切换权限后，请查看左侧导航栏的变化</p>
        </CardContent>
      </Card>
    </div>
  )
}