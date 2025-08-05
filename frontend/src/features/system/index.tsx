'use client'

import { useState } from 'react'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { SystemSettings } from './components/system-settings'
import SystemProvider from './context/system-context'

function SystemContent() {
  return (
    <div className='-mx-4 flex-1 overflow-auto px-4 py-1 lg:flex-row lg:space-y-0 lg:space-x-12'>
      <SystemSettings />
    </div>
  )
}

export default function SystemManagement() {
  return (
    <SystemProvider>
      <Header fixed>
        <Search />
        <div className='ml-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      <Main>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>系统管理</h2>
            <p className='text-muted-foreground'>
              管理系统配置和选项。
            </p>
          </div>
        </div>
        <SystemContent />
      </Main>
    </SystemProvider>
  )
}