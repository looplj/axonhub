import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import {
  missingRequiredConfig,
  useEnablePlugin,
  useInstallPlugin,
  useInstallPluginFromMarket,
  usePlugins,
  type Plugin,
  type PluginInstallResult,
  type PluginUpgradePreview,
} from '../data/plugins';

export type PluginInstallSource = { kind: 'file'; file: File } | { kind: 'market'; pluginID: string; version: string };

export type PluginGrantMode = 'install' | 'upgrade' | 'edit';

interface GrantState {
  mode: PluginGrantMode;
  plugin: Plugin;
  preview: PluginUpgradePreview | null;
  // Upgrades are resubmitted with the complete grant sets, so the original package source is kept.
  source: PluginInstallSource | null;
}

/**
 * Drives upgrade, enable, and permission edits through one grant dialog.
 * A new plugin is saved disabled so the owner can configure it before enabling it from the card;
 * an upgrade of an enabled plugin that adds capabilities returns a preview and is applied only when
 * the grants are resubmitted.
 */
export function usePluginGrantFlow() {
  const { t } = useTranslation();
  const pluginsQuery = usePlugins();
  const installPlugin = useInstallPlugin();
  const installFromMarket = useInstallPluginFromMarket();
  const enablePlugin = useEnablePlugin();
  const [grant, setGrant] = useState<GrantState | null>(null);

  const submit = (source: PluginInstallSource, grantedPermissions: string[] | null, grantedNetwork: string[] | null) =>
    source.kind === 'file'
      ? installPlugin.mutateAsync({ file: source.file, grantedPermissions, grantedNetwork })
      : installFromMarket.mutateAsync({ pluginID: source.pluginID, version: source.version, grantedPermissions, grantedNetwork });

  const showPreview = (result: PluginInstallResult, source: PluginInstallSource) => {
    if (!result.upgradePreview || !result.plugin) {
      return false;
    }
    setGrant({ mode: 'upgrade', plugin: result.plugin, preview: result.upgradePreview, source });
    return true;
  };

  /** Resolves true when the package was accepted, either applied directly or waiting on the grant dialog. */
  const install = async (source: PluginInstallSource, successMessage: string) => {
    const known = new Set((pluginsQuery.data?.plugins ?? []).map((plugin) => plugin.pluginID));
    try {
      const result = await submit(source, null, null);
      if (showPreview(result, source)) {
        return true;
      }
      if (result.plugin && !known.has(result.plugin.pluginID) && !result.plugin.enabled) {
        toast.success(t('plugins.installedDisabled', { name: result.plugin.name || result.plugin.pluginID }));
        return true;
      }
      toast.success(successMessage);
      return true;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t(source.kind === 'file' ? 'plugins.upload.failed' : 'plugins.market.failed'));
      return false;
    }
  };

  /** Returns false without opening the grant dialog while required config is missing. */
  const enable = (plugin: Plugin) => {
    const missing = missingRequiredConfig(plugin);
    if (missing.length > 0) {
      toast.error(t('plugins.config.requiredMissing', { fields: missing.map((field) => field.title || field.name).join(', ') }));
      return false;
    }
    setGrant({ mode: 'install', plugin, preview: null, source: null });
    return true;
  };

  const editPermissions = (plugin: Plugin) => setGrant({ mode: 'edit', plugin, preview: null, source: null });

  const confirm = async (grantedPermissions: string[], grantedNetwork: string[]) => {
    if (!grant) {
      return;
    }
    if (grant.mode === 'upgrade') {
      if (!grant.source) {
        return;
      }
      try {
        const result = await submit(grant.source, grantedPermissions, grantedNetwork);
        // The package changed again between preview and confirm; show the fresh diff.
        if (showPreview(result, grant.source)) {
          return;
        }
        toast.success(t('plugins.market.upgraded'));
        setGrant(null);
      } catch (error) {
        toast.error(error instanceof Error ? error.message : t('plugins.actions.failed'));
      }
      return;
    }
    try {
      // useEnablePlugin reports its own errors.
      await enablePlugin.mutateAsync({ id: grant.plugin.id, grantedPermissions, grantedNetwork });
      toast.success(grant.mode === 'edit' ? t('plugins.grant.saved') : t('plugins.actions.enabled'));
      setGrant(null);
    } catch {
      // Keep the dialog open so the owner can adjust the selection and retry.
    }
  };

  const cancel = () => setGrant(null);

  const pending = installPlugin.isPending || installFromMarket.isPending || enablePlugin.isPending;

  return {
    grant,
    pending,
    // Market rows and the upload dialog lock while any step of a flow is unresolved.
    busy: pending || grant !== null,
    install,
    enable,
    editPermissions,
    confirm,
    cancel,
  };
}
