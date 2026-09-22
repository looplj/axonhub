// Shared override operation row editors.
// Used by the per-channel override dialog and the standalone template manager.
import { useState } from 'react';
import { useWatch, Control } from 'react-hook-form';
import { ChevronDown, ChevronUp } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { OverrideOperation } from '../data/schema';
import {
  AUTH_HEADER_KEYS,
  BODY_OP_TYPES,
  HEADER_OP_TYPES,
  OP_LABELS,
  OverrideFormValues,
  OpType,
  isArrayInsertOp,
  isArrayOp,
  parseValueForDisplay,
} from '../utils/override-operations';

interface OperationRowProps {
  index: number;
  control: Control<OverrideFormValues>;
  fieldName: 'bodyOverrideOperations' | 'headerOverrideOperations';
  onUpdate: (index: number, data: Partial<OverrideOperation>) => void;
  onRemove: (index: number) => void;
}

export function OperationRow({ index, control, fieldName, onUpdate, onRemove }: OperationRowProps) {
  const { t } = useTranslation();
  const field = useWatch({ control, name: `${fieldName}.${index}` }) as OverrideOperation;
  const [showCondition, setShowCondition] = useState(!!field?.condition);

  if (!field) return null;

  const arrayOp = isArrayOp(field.op);
  const arrayInsertOp = isArrayInsertOp(field.op);
  const needsPathOnly = field.op === 'set' || field.op === 'set_if_absent' || field.op === 'delete' || arrayOp;
  const needsFromTo = field.op === 'rename' || field.op === 'copy';
  const needsValue = field.op === 'set' || field.op === 'set_if_absent' || arrayInsertOp;
  const needsIndex = field.op === 'array_insert';
  const needsMatch = field.op === 'array_remove';

  return (
    <div className='space-y-3 rounded-lg border p-3'>
      <div className='flex items-center gap-3'>
        <div className='w-36'>
          <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.body.op')}</Label>
          <Select
            value={field.op}
            onValueChange={(v) => onUpdate(index, { op: v as OpType })}
          >
            <SelectTrigger data-testid={`op-type-${index}`} className='mt-1'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {BODY_OP_TYPES.map((opType) => (
                <SelectItem key={opType} value={opType}>
                  {t(OP_LABELS[opType])}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {needsPathOnly && (
          <div className='flex-1'>
            <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.body.path')}</Label>
            <Input
              data-testid={`op-path-${index}`}
              className='mt-1 font-mono'
              placeholder={t(arrayOp
                ? 'channels.dialogs.settings.overrides.body.arrayPathPlaceholder'
                : 'channels.dialogs.settings.overrides.body.pathPlaceholder')}
              value={field.path || ''}
              onChange={(e) => onUpdate(index, { path: e.target.value })}
            />
          </div>
        )}

        {needsFromTo && (
          <>
            <div className='flex-1'>
              <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.body.from')}</Label>
              <Input
                data-testid={`op-from-${index}`}
                className='mt-1 font-mono'
                placeholder={t('channels.dialogs.settings.overrides.body.fromPlaceholder')}
                value={field.from || ''}
                onChange={(e) => onUpdate(index, { from: e.target.value })}
              />
            </div>
            <div className='flex-1'>
              <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.body.to')}</Label>
              <Input
                data-testid={`op-to-${index}`}
                className='mt-1 font-mono'
                placeholder={t('channels.dialogs.settings.overrides.body.toPlaceholder')}
                value={field.to || ''}
                onChange={(e) => onUpdate(index, { to: e.target.value })}
              />
            </div>
          </>
        )}

        {needsIndex && (
          <div className='w-32'>
            <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.body.arrayIndex')}</Label>
            <Input
              data-testid={`op-index-${index}`}
              type='number'
              className='mt-1 font-mono'
              placeholder={t('channels.dialogs.settings.overrides.body.arrayIndexPlaceholder')}
              value={typeof field.index === 'number' ? String(field.index) : ''}
              onChange={(e) => {
                const raw = e.target.value;
                if (raw === '' || raw === '-') {
                  onUpdate(index, { index: undefined });
                  return;
                }
                const parsed = Number(raw);
                if (!Number.isNaN(parsed) && Number.isFinite(parsed)) {
                  onUpdate(index, { index: Math.trunc(parsed) });
                }
              }}
            />
          </div>
        )}

        <div className='pt-5'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => onRemove(index)}
            className='px-3'
            data-testid={`remove-op-${index}`}
          >
            {t('common.buttons.remove')}
          </Button>
        </div>
      </div>

      {needsValue && (
        <div>
          <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.body.value')}</Label>
          <Input
            data-testid={`op-value-${index}`}
            className='mt-1 font-mono'
            placeholder={t(arrayInsertOp
              ? 'channels.dialogs.settings.overrides.body.arrayValuePlaceholder'
              : 'channels.dialogs.settings.overrides.body.valuePlaceholder')}
            value={parseValueForDisplay(field.value)}
            onChange={(e) => onUpdate(index, { value: e.target.value })}
          />
          {arrayInsertOp && (
            <label className='text-muted-foreground mt-2 flex items-center gap-2 text-xs'>
              <input
                type='checkbox'
                data-testid={`op-splat-${index}`}
                checked={field.splat !== false}
                onChange={(e) => onUpdate(index, { splat: e.target.checked })}
              />
              <span>{t('channels.dialogs.settings.overrides.body.arraySplatLabel')}</span>
            </label>
          )}
        </div>
      )}

      {needsMatch && (
        <div className='grid gap-3 sm:grid-cols-2'>
          <div>
            <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.body.matchPath')}</Label>
            <Input
              data-testid={`op-match-path-${index}`}
              className='mt-1 font-mono'
              placeholder={t('channels.dialogs.settings.overrides.body.matchPathPlaceholder')}
              value={field.match?.path || ''}
              onChange={(e) => onUpdate(index, { match: { path: e.target.value, eq: field.match?.eq || '' } })}
            />
          </div>
          <div>
            <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.body.matchEq')}</Label>
            <Input
              data-testid={`op-match-eq-${index}`}
              className='mt-1 font-mono'
              placeholder={t('channels.dialogs.settings.overrides.body.matchEqPlaceholder')}
              value={field.match?.eq || ''}
              onChange={(e) => onUpdate(index, { match: { path: field.match?.path || '', eq: e.target.value } })}
            />
          </div>
        </div>
      )}

      <div>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          className='text-muted-foreground h-6 px-1 text-xs'
          onClick={() => setShowCondition(!showCondition)}
        >
          {showCondition ? <ChevronUp className='mr-1 h-3 w-3' /> : <ChevronDown className='mr-1 h-3 w-3' />}
          {t('channels.dialogs.settings.overrides.body.condition')}
        </Button>
        {showCondition && (
          <Input
            data-testid={`op-condition-${index}`}
            className='mt-1 font-mono text-sm'
            placeholder={t('channels.dialogs.settings.overrides.body.conditionPlaceholder')}
            value={field.condition || ''}
            onChange={(e) => onUpdate(index, { condition: e.target.value })}
          />
        )}
      </div>
    </div>
  );
}

interface HeaderOperationRowProps {
  index: number;
  control: Control<OverrideFormValues>;
  onUpdate: (index: number, data: Partial<OverrideOperation>) => void;
  onRemove: (index: number) => void;
}

export function HeaderOperationRow({ index, control, onUpdate, onRemove }: HeaderOperationRowProps) {
  const { t } = useTranslation();
  const field = useWatch({ control, name: `headerOverrideOperations.${index}` }) as OverrideOperation;
  const [showCondition, setShowCondition] = useState(!!field?.condition);

  if (!field) return null;

  const opType = field.op;
  const needsPathOnly = opType === 'set' || opType === 'delete';
  const needsFromTo = opType === 'rename' || opType === 'copy';
  const needsValue = opType === 'set';

  const normalizedKey = (field.path || '').trim().toLowerCase();
  const isAuthHeader = normalizedKey !== '' && AUTH_HEADER_KEYS.includes(normalizedKey);

  return (
    <div className='space-y-3 rounded-lg border p-3'>
      <div className='flex items-center gap-3'>
        <div className='w-36'>
          <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.body.op')}</Label>
          <Select
            value={opType}
            onValueChange={(v) => onUpdate(index, { op: v as OpType })}
          >
            <SelectTrigger data-testid={`header-op-type-${index}`} className='mt-1'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {HEADER_OP_TYPES.map((opType) => (
                <SelectItem key={opType} value={opType}>
                  {t(OP_LABELS[opType])}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {needsPathOnly && (
          <div className='flex-1'>
            <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.headers.key')}</Label>
            <Input
              data-testid={`header-op-path-${index}`}
              className='mt-1 font-mono'
              placeholder={t('channels.dialogs.settings.overrides.headers.keyPlaceholder')}
              value={field.path || ''}
              onChange={(e) => onUpdate(index, { path: e.target.value })}
            />
            {isAuthHeader && (
              <p className='text-destructive mt-1 text-sm' role='alert'>
                {t('channels.dialogs.settings.overrides.headers.sensitiveWarning')}
              </p>
            )}
          </div>
        )}

        {needsFromTo && (
          <>
            <div className='flex-1'>
              <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.headers.from')}</Label>
              <Input
                data-testid={`header-op-from-${index}`}
                className='mt-1 font-mono'
                placeholder={t('channels.dialogs.settings.overrides.headers.fromPlaceholder')}
                value={field.from || ''}
                onChange={(e) => onUpdate(index, { from: e.target.value })}
              />
            </div>
            <div className='flex-1'>
              <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.headers.to')}</Label>
              <Input
                data-testid={`header-op-to-${index}`}
                className='mt-1 font-mono'
                placeholder={t('channels.dialogs.settings.overrides.headers.toPlaceholder')}
                value={field.to || ''}
                onChange={(e) => onUpdate(index, { to: e.target.value })}
              />
            </div>
          </>
        )}

        <div className='pt-5'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => onRemove(index)}
            className='px-3'
            data-testid={`remove-header-op-${index}`}
          >
            {t('common.buttons.remove')}
          </Button>
        </div>
      </div>

      {needsValue && (
        <div>
          <Label className='text-sm font-medium'>{t('channels.dialogs.settings.overrides.headers.value')}</Label>
          <Input
            data-testid={`header-op-value-${index}`}
            className='mt-1 font-mono'
            placeholder={t('channels.dialogs.settings.overrides.headers.valuePlaceholder')}
            value={parseValueForDisplay(field.value)}
            onChange={(e) => onUpdate(index, { value: e.target.value })}
          />
        </div>
      )}

      <div>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          className='text-muted-foreground h-6 px-1 text-xs'
          onClick={() => setShowCondition(!showCondition)}
        >
          {showCondition ? <ChevronUp className='mr-1 h-3 w-3' /> : <ChevronDown className='mr-1 h-3 w-3' />}
          {t('channels.dialogs.settings.overrides.body.condition')}
        </Button>
        {showCondition && (
          <Input
            data-testid={`header-op-condition-${index}`}
            className='mt-1 font-mono text-sm'
            placeholder={t('channels.dialogs.settings.overrides.body.conditionPlaceholder')}
            value={field.condition || ''}
            onChange={(e) => onUpdate(index, { condition: e.target.value })}
          />
        )}
      </div>
    </div>
  );
}
