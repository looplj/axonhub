'use client';

import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { PLUGIN_SECRET_MASK, useUpdatePluginSettings, type Plugin, type PluginConfigField } from '../data/plugins';

interface PluginConfigFormProps {
  plugin: Plugin;
}

export function PluginConfigForm({ plugin }: PluginConfigFormProps) {
  const { t } = useTranslation();
  const updateSettings = useUpdatePluginSettings();
  const [priority, setPriority] = useState(String(plugin.priority));
  const [values, setValues] = useState<Record<string, string | boolean>>({});
  const [secrets, setSecrets] = useState<Record<string, { edited: boolean; value: string }>>({});
  const snapshot = JSON.stringify({
    id: plugin.id,
    priority: plugin.priority,
    config: plugin.config,
    fields: plugin.configFields,
  });

  useEffect(() => {
    const saved = JSON.parse(snapshot) as {
      priority: number;
      config: Plugin['config'];
      fields: Plugin['configFields'];
    };
    const nextValues: Record<string, string | boolean> = {};
    const nextSecrets: Record<string, { edited: boolean; value: string }> = {};
    for (const field of saved.fields) {
      if (field.format === 'secret') {
        nextSecrets[field.name] = { edited: false, value: '' };
        continue;
      }
      nextValues[field.name] = initialFieldValue(field, saved.config);
    }
    setValues(nextValues);
    setSecrets(nextSecrets);
    setPriority(String(saved.priority));
  }, [snapshot]);

  const save = () => {
    const parsedPriority = Number.parseInt(priority, 10);
    if (Number.isNaN(parsedPriority)) {
      return;
    }
    const config: Record<string, unknown> = {};
    for (const field of plugin.configFields) {
      if (field.format === 'secret') {
        const secret = secrets[field.name];
        config[field.name] = secret?.edited && secret.value !== '' ? secret.value : PLUGIN_SECRET_MASK;
        continue;
      }
      const raw = values[field.name];
      if (field.type === 'boolean') {
        config[field.name] = raw === true || raw === 'true';
        continue;
      }
      if (raw === undefined || (raw === '' && field.type !== 'string')) {
        continue;
      }
      if (field.type === 'integer') {
        const parsed = Number.parseInt(String(raw), 10);
        if (!Number.isNaN(parsed)) {
          config[field.name] = parsed;
        }
        continue;
      }
      if (field.type === 'number') {
        const parsed = Number(raw);
        if (!Number.isNaN(parsed)) {
          config[field.name] = parsed;
        }
        continue;
      }
      config[field.name] = String(raw);
    }
    updateSettings.mutate({ id: plugin.id, priority: parsedPriority, config });
  };

  return (
    <div className='space-y-4'>
      <div>
        <h3 className='text-sm font-medium'>{t('plugins.config.title')}</h3>
        <p className='text-muted-foreground text-sm'>{t('plugins.config.description')}</p>
      </div>
      <div className='space-y-2'>
        <Label htmlFor={`plugin-priority-${plugin.id}`}>{t('plugins.config.priority')}</Label>
        <Input
          id={`plugin-priority-${plugin.id}`}
          type='number'
          value={priority}
          onChange={(event) => setPriority(event.target.value)}
        />
      </div>
      {plugin.configFields.length === 0 && <p className='text-muted-foreground text-sm'>{t('plugins.config.empty')}</p>}
      {plugin.configFields.map((field) => (
        <ConfigInput
          key={field.name}
          pluginID={plugin.id}
          field={field}
          value={values[field.name]}
          secretValue={secrets[field.name]?.value ?? ''}
          onValueChange={(value) => setValues((current) => ({ ...current, [field.name]: value }))}
          onSecretChange={(value) =>
            setSecrets((current) => ({ ...current, [field.name]: { edited: true, value } }))
          }
        />
      ))}
      <Button type='button' onClick={save} disabled={updateSettings.isPending}>
        {updateSettings.isPending ? t('plugins.config.saving') : t('plugins.config.save')}
      </Button>
    </div>
  );
}

function ConfigInput({
  pluginID,
  field,
  value,
  secretValue,
  onValueChange,
  onSecretChange,
}: {
  pluginID: string;
  field: PluginConfigField;
  value: string | boolean | undefined;
  secretValue: string;
  onValueChange: (value: string | boolean) => void;
  onSecretChange: (value: string) => void;
}) {
  const { t } = useTranslation();
  const inputID = `plugin-${pluginID}-${field.name}`;
  const label = field.title || field.name;

  return (
    <div className='space-y-2'>
      <Label htmlFor={inputID}>{label}</Label>
      {field.description && <p className='text-muted-foreground text-sm'>{field.description}</p>}
      {field.format === 'secret' ? (
        <Input
          id={inputID}
          type='password'
          autoComplete='new-password'
          value={secretValue}
          placeholder={t('plugins.config.secretPlaceholder')}
          onChange={(event) => onSecretChange(event.target.value)}
        />
      ) : field.enumValues.length > 0 ? (
        <Select value={value !== undefined && value !== '' ? String(value) : undefined} onValueChange={onValueChange}>
          <SelectTrigger id={inputID}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {field.enumValues.map((item) => (
              <SelectItem key={item} value={item}>
                {item}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : field.type === 'boolean' ? (
        <Switch id={inputID} checked={value === true} onCheckedChange={onValueChange} />
      ) : (
        <Input
          id={inputID}
          type={field.type === 'integer' || field.type === 'number' ? 'number' : 'text'}
          step={field.type === 'integer' ? 1 : undefined}
          min={field.minimum ?? undefined}
          max={field.maximum ?? undefined}
          value={typeof value === 'string' ? value : ''}
          onChange={(event) => onValueChange(event.target.value)}
        />
      )}
    </div>
  );
}

function initialFieldValue(field: PluginConfigField, config: Record<string, unknown>): string | boolean {
  const current = config[field.name];
  if (field.type === 'boolean') {
    if (typeof current === 'boolean') {
      return current;
    }
    if (field.defaultJSON) {
      try {
        return JSON.parse(field.defaultJSON) === true;
      } catch {
        return false;
      }
    }
    return false;
  }
  if (current !== undefined && current !== null) {
    return String(current);
  }
  if (field.defaultJSON) {
    try {
      const parsed = JSON.parse(field.defaultJSON);
      return parsed === null || parsed === undefined ? '' : String(parsed);
    } catch {
      return '';
    }
  }
  return '';
}
