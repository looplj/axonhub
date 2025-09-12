'use client'

import React from 'react'
import { SystemSettingsTabs } from './tabs'

// This component is kept for backward compatibility but is no longer used
// The tabbed interface is now implemented in SystemSettingsTabs
export function SystemSettings() {
  return <SystemSettingsTabs />
}
