import { AboutSettings } from './components/about-settings';
import { BackupSettings } from './components/backup-settings';
import { BrandSettings } from './components/brand-settings';
import { ClaudeCodeSettings } from './components/claude-code-settings';
import { CodexCompatibilitySettings } from './components/codex-compatibility-settings';
import { DiagnosticsSettings } from './components/diagnostics-settings';
import { ForwardingSettings } from './components/forwarding-settings';
import { GeneralSettings } from './components/general-settings';
import { ProxyPresetsSettings } from './components/proxy-presets-settings';
import { QuotaSettings } from './components/quota-settings';
import { RetrySettings } from './components/retry-settings';
import { SecuritySettings } from './components/security-settings';
import { StorageSettings } from './components/storage-settings';
import { WebhookSettings } from './components/webhook-settings';

// System settings are split into a few pages, each listed under the admin
// "Settings" sidebar group and showing its settings as tabs. `general` lives at
// /system, the others at /system/<page>.
export type SystemPageKey = 'general' | 'requests' | 'integrations' | 'operations';

export type SystemTabKey =
  | 'general'
  | 'security'
  | 'brand'
  | 'about'
  | 'retry'
  | 'forwarding'
  | 'proxy'
  | 'quota'
  | 'webhook'
  | 'claude-code'
  | 'codex'
  | 'storage'
  | 'diagnostics'
  | 'backup';

export interface SystemTabDef {
  key: SystemTabKey;
  labelKey: string;
  component: React.ComponentType;
  ownerOnly?: boolean;
}

export interface SystemPageDef {
  titleKey: string;
  descriptionKey: string;
  tabs: SystemTabDef[];
}

export const SYSTEM_PAGES: Record<SystemPageKey, SystemPageDef> = {
  general: {
    titleKey: 'system.title',
    descriptionKey: 'system.description',
    tabs: [
      { key: 'general', labelKey: 'system.tabs.general', component: GeneralSettings },
      { key: 'security', labelKey: 'system.tabs.security', component: SecuritySettings },
      { key: 'brand', labelKey: 'system.tabs.brand', component: BrandSettings },
      { key: 'about', labelKey: 'system.tabs.about', component: AboutSettings },
    ],
  },
  requests: {
    titleKey: 'system.pages.requests.title',
    descriptionKey: 'system.pages.requests.description',
    tabs: [
      { key: 'retry', labelKey: 'system.tabs.retry', component: RetrySettings },
      { key: 'forwarding', labelKey: 'system.tabs.forwarding', component: ForwardingSettings },
      { key: 'proxy', labelKey: 'system.tabs.proxy', component: ProxyPresetsSettings },
      { key: 'quota', labelKey: 'system.tabs.quota', component: QuotaSettings },
    ],
  },
  integrations: {
    titleKey: 'system.pages.integrations.title',
    descriptionKey: 'system.pages.integrations.description',
    tabs: [
      { key: 'webhook', labelKey: 'system.tabs.webhook', component: WebhookSettings },
      { key: 'claude-code', labelKey: 'system.tabs.claudeCode', component: ClaudeCodeSettings },
      { key: 'codex', labelKey: 'system.tabs.codex', component: CodexCompatibilitySettings },
    ],
  },
  operations: {
    titleKey: 'system.pages.operations.title',
    descriptionKey: 'system.pages.operations.description',
    tabs: [
      { key: 'storage', labelKey: 'system.tabs.storage', component: StorageSettings },
      { key: 'diagnostics', labelKey: 'system.tabs.diagnostics', component: DiagnosticsSettings, ownerOnly: true },
      { key: 'backup', labelKey: 'system.tabs.backup', component: BackupSettings, ownerOnly: true },
    ],
  },
};

export function isSystemSubPageKey(value: string | undefined): value is Exclude<SystemPageKey, 'general'> {
  return value !== undefined && value !== 'general' && Object.prototype.hasOwnProperty.call(SYSTEM_PAGES, value);
}
