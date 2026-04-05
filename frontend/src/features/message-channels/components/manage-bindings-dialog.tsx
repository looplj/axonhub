import { useEffect, useState, useRef } from 'react';
import { IconPlus, IconTrash, IconChevronDown, IconChevronUp, IconLink, IconUser, IconUsers, IconCheck } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { extractNumberIDAsNumber } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import { useQueryAgentInstances } from '@/features/agents/data/agent-instances';
import { useBatchSaveMessageChannelBindings, useCreateBindingRequest, useBindingRequestStatus, type BatchBindingInput } from '../data/message-channels';
import type { MessageChannel, MessageChannelAgentInstanceBindingInput, MessageChatType } from '../data/schema';

interface ManageBindingsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: MessageChannel;
  onRefresh?: () => void;
}

interface BindingFormData {
  id?: string;
  agentInstanceID: string;
  agentInstanceName: string;
  enabled: boolean;
  config: MessageChannelAgentInstanceBindingInput;
  expanded?: boolean;
}

export function ManageBindingsDialog({ open, onOpenChange, currentRow, onRefresh }: ManageBindingsDialogProps) {
  const { t } = useTranslation();
  const [bindings, setBindings] = useState<BindingFormData[]>([]);
  const [selectedAgentId, setSelectedAgentId] = useState<string>('');
  const [showPairCodeDialog, setShowPairCodeDialog] = useState(false);
  const [generatedPairCode, setGeneratedPairCode] = useState<string>('');
  const [hasPendingPairCode, setHasPendingPairCode] = useState(false);
  const [saveDisabled, setSaveDisabled] = useState(false);
  const pairedRef = useRef(false);

  const { data: agentInstancesData, isLoading } = useQueryAgentInstances({
    first: 100,
  });

  const saveBindingsMutation = useBatchSaveMessageChannelBindings();
  const createRequestMutation = useCreateBindingRequest();

  const { data: bindingRequestStatus } = useBindingRequestStatus(
    generatedPairCode,
    { enabled: showPairCodeDialog && hasPendingPairCode }
  );

  useEffect(() => {
    if (bindingRequestStatus?.status === 'approved' && !pairedRef.current) {
      pairedRef.current = true;
      setHasPendingPairCode(false);
      setSaveDisabled(true);
      toast.success(t('messageChannels.messages.pairingSuccess'));
      onRefresh?.();
      setShowPairCodeDialog(false);
    }
  }, [bindingRequestStatus, t, onRefresh]);

  useEffect(() => {
    if (open) {
      const edges = currentRow.agentInstanceBindings?.edges || [];
      const existingBindings = edges.map((edge) => ({
        id: edge.node.id,
        agentInstanceID: edge.node.agentInstanceID,
        agentInstanceName: edge.node.agentInstance.name,
        enabled: edge.node.enabled,
        config: edge.node.config || { chatType: 'dm', chatID: '', allowFrom: [], excludeKeywords: [], allowWithoutMention: false },
        expanded: false,
      }));
      setBindings(existingBindings);
      setSaveDisabled(false);
    } else {
      setBindings([]);
      setGeneratedPairCode('');
      setHasPendingPairCode(false);
      setSaveDisabled(false);
      pairedRef.current = false;
    }
  }, [open, currentRow]);

  const agentInstances = agentInstancesData?.edges?.map((edge) => edge.node) || [];
  const boundAgentIds = new Set(bindings.map((b) => b.agentInstanceID));
  const availableAgents = agentInstances.filter((agent) => !boundAgentIds.has(agent.id));

  const handleAddBinding = () => {
    if (!selectedAgentId) return;
    const agent = agentInstances.find((a) => a.id === selectedAgentId);
    if (!agent) return;

    setBindings((prev) => [
      ...prev,
      {
        agentInstanceID: agent.id,
        agentInstanceName: agent.name || t('messageChannels.dialogs.manageBindings.instancePrefix', { id: agent.id.slice(0, 8) }),
        enabled: true,
        config: { chatType: 'dm', chatID: '', allowFrom: [], excludeKeywords: [], allowWithoutMention: false },
        expanded: true,
      },
    ]);
    setSelectedAgentId('');
  };

  const handleRemoveBinding = (index: number) => {
    setBindings((prev) => prev.filter((_, i) => i !== index));
  };

  const handleToggleEnabled = (index: number) => {
    setBindings((prev) => prev.map((b, i) => (i === index ? { ...b, enabled: !b.enabled } : b)));
  };

  const handleToggleExpanded = (index: number) => {
    setBindings((prev) => prev.map((b, i) => (i === index ? { ...b, expanded: !b.expanded } : b)));
  };

  const handleConfigChange = (
    index: number,
    field: keyof MessageChannelAgentInstanceBindingInput,
    value: string | string[] | MessageChatType | boolean
  ) => {
    setBindings((prev) => prev.map((b, i) => (i === index ? { ...b, config: { ...b.config, [field]: value } } : b)));
  };

  const handleAddConfigItem = (index: number, field: 'allowFrom' | 'excludeKeywords') => {
    setBindings((prev) =>
      prev.map((b, i) => (i === index ? { ...b, config: { ...b.config, [field]: [...(b.config[field] || []), ''] } } : b))
    );
  };

  const handleRemoveConfigItem = (index: number, field: 'allowFrom' | 'excludeKeywords', itemIndex: number) => {
    setBindings((prev) =>
      prev.map((b, i) =>
        i === index
          ? {
              ...b,
              config: {
                ...b.config,
                [field]: (b.config[field] || []).filter((_, j) => j !== itemIndex),
              },
            }
          : b
      )
    );
  };

  const handleConfigItemChange = (index: number, field: 'allowFrom' | 'excludeKeywords', itemIndex: number, value: string) => {
    setBindings((prev) =>
      prev.map((b, i) =>
        i === index
          ? {
              ...b,
              config: {
                ...b.config,
                [field]: (b.config[field] || []).map((item, j) => (j === itemIndex ? value : item)),
              },
            }
          : b
      )
    );
  };

  const handleSave = async () => {
    const inputBindings: BatchBindingInput[] = bindings.map((b) => ({
      agentInstanceID: b.agentInstanceID,
      enabled: b.enabled,
      config: b.config,
    }));

    saveBindingsMutation.mutate(
      {
        messageChannelID: currentRow.id,
        bindings: inputBindings,
      },
      {
        onSuccess: () => {
          onOpenChange(false);
        },
      }
    );
  };

  const handleGeneratePairCode = async () => {
    if (!selectedAgentId) return;

    createRequestMutation.mutate(
      {
        messageChannelID: extractNumberIDAsNumber(currentRow.id),
        agentInstanceID: extractNumberIDAsNumber(selectedAgentId),
        type: 'pair',
      },
      {
        onSuccess: (data) => {
          setGeneratedPairCode(data.pairCode);
          setHasPendingPairCode(true);
          pairedRef.current = false;
          setShowPairCodeDialog(true);
        },
      }
    );
  };

  const getAgentDisplayName = (agent: { id: string; name?: string | null; status?: string | null }) => {
    const name = agent.name || t('messageChannels.dialogs.manageBindings.instancePrefix', { id: agent.id.slice(0, 8) });
    return agent.status ? `${name} (${agent.status})` : name;
  };

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className='flex h-[85vh] max-h-[800px] flex-col sm:max-w-[650px]'>
          <DialogHeader className='shrink-0 text-left'>
            <DialogTitle>{t('messageChannels.dialogs.manageBindings.title')}</DialogTitle>
            <DialogDescription>{t('messageChannels.dialogs.manageBindings.description', { name: currentRow.name })}</DialogDescription>
          </DialogHeader>

          <div className='flex min-h-0 flex-1 flex-col gap-4'>
            <div className='flex flex-col gap-3 sm:flex-row sm:items-end'>
              <div className='flex-1 space-y-1.5'>
                <Label className='text-sm font-medium'>{t('messageChannels.dialogs.manageBindings.selectAgent')}</Label>
                <Select value={selectedAgentId} onValueChange={setSelectedAgentId} disabled={isLoading}>
                  <SelectTrigger>
                    <SelectValue placeholder={t('messageChannels.dialogs.manageBindings.selectPlaceholder')} />
                  </SelectTrigger>
                  <SelectContent>
                    {availableAgents.length === 0 && (
                      <SelectItem value='none' disabled>
                        {t('messageChannels.dialogs.manageBindings.noAvailableAgents')}
                      </SelectItem>
                    )}
                    {availableAgents.map((agent) => (
                      <SelectItem key={agent.id} value={agent.id}>
                        {getAgentDisplayName(agent)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className='flex gap-2'>
                <Button type='button' onClick={handleAddBinding} disabled={!selectedAgentId || availableAgents.length === 0}>
                  <IconPlus className='mr-2 h-4 w-4' />
                  {t('common.buttons.add')}
                </Button>
                <Button
                  type='button'
                  variant='outline'
                  onClick={handleGeneratePairCode}
                  disabled={!selectedAgentId || createRequestMutation.isPending}
                >
                  <IconLink className='mr-2 h-4 w-4' />
                  {createRequestMutation.isPending
                    ? t('common.buttons.generating')
                    : t('messageChannels.dialogs.bindingRequest.generate')}
                </Button>
              </div>
            </div>

            <Separator />

            <div className='flex-1 overflow-hidden rounded-lg border'>
              <ScrollArea className='h-full'>
                {bindings.length === 0 ? (
                  <div className='text-muted-foreground flex h-32 items-center justify-center'>
                    {t('messageChannels.dialogs.manageBindings.noBindings')}
                  </div>
                ) : (
                  <div className='space-y-2 p-3'>
                    {bindings.map((binding, index) => (
                      <Collapsible
                        key={binding.agentInstanceID}
                        open={binding.expanded}
                        onOpenChange={() => handleToggleExpanded(index)}
                        className='bg-card overflow-hidden rounded-lg border'
                      >
                        <div className='flex items-center gap-3 px-3 py-2'>
                          <div className='flex min-w-0 flex-1 items-center gap-2'>
                            {binding.config.chatType === 'dm' ? (
                              <IconUser className='text-muted-foreground h-4 w-4 shrink-0' />
                            ) : (
                              <IconUsers className='text-muted-foreground h-4 w-4 shrink-0' />
                            )}
                            <span className='truncate text-sm font-medium'>{binding.agentInstanceName}</span>
                          </div>

                          <div className='text-muted-foreground hidden items-center gap-2 text-xs sm:flex'>
                            <Badge variant='secondary' className='font-normal'>
                              {t(`messageChannels.dialogs.manageBindings.config.${binding.config.chatType || 'dm'}`)}
                            </Badge>
                            {binding.config.chatID && (
                              <code className='bg-muted max-w-[100px] truncate rounded px-1.5 py-0.5 font-mono text-[10px]'>
                                {binding.config.chatID}
                              </code>
                            )}
                          </div>

                          <div className='flex shrink-0 items-center gap-1'>
                            <Switch checked={binding.enabled} onCheckedChange={() => handleToggleEnabled(index)} className='scale-75' />

                            <CollapsibleTrigger asChild>
                              <Button type='button' variant='ghost' size='icon' className='h-7 w-7'>
                                {binding.expanded ? <IconChevronUp className='h-4 w-4' /> : <IconChevronDown className='h-4 w-4' />}
                              </Button>
                            </CollapsibleTrigger>

                            <Button
                              type='button'
                              variant='ghost'
                              size='icon'
                              className='text-destructive hover:text-destructive hover:bg-destructive/10 h-7 w-7'
                              onClick={() => handleRemoveBinding(index)}
                            >
                              <IconTrash className='h-4 w-4' />
                            </Button>
                          </div>
                        </div>

                        <CollapsibleContent>
                          <div className='bg-muted/30 space-y-4 border-t px-3 py-3'>
                            <div className='grid grid-cols-2 gap-4'>
                              <div className='space-y-1.5'>
                                <Label className='text-xs font-medium'>
                                  {t('messageChannels.dialogs.manageBindings.config.chatType')}
                                </Label>
                                <Select
                                  value={binding.config.chatType || 'dm'}
                                  onValueChange={(value) => handleConfigChange(index, 'chatType', value as MessageChatType)}
                                >
                                  <SelectTrigger className='h-8 text-sm'>
                                    <SelectValue />
                                  </SelectTrigger>
                                  <SelectContent>
                                    <SelectItem value='dm'>{t('messageChannels.dialogs.manageBindings.config.dm')}</SelectItem>
                                    <SelectItem value='group'>{t('messageChannels.dialogs.manageBindings.config.group')}</SelectItem>
                                  </SelectContent>
                                </Select>
                              </div>

                              <div className='space-y-1.5'>
                                <Label className='text-xs font-medium'>{t('messageChannels.dialogs.manageBindings.config.chatID')}</Label>
                                <Input
                                  value={binding.config.chatID || ''}
                                  onChange={(e) => handleConfigChange(index, 'chatID', e.target.value)}
                                  placeholder={t('messageChannels.dialogs.manageBindings.config.chatIDPlaceholder')}
                                  className='h-8 text-sm'
                                />
                              </div>
                            </div>

                            {binding.config.chatType === 'group' && (
                              <div className='flex items-center justify-between rounded-md border px-3 py-2'>
                                <div className='space-y-0.5'>
                                  <Label className='text-xs font-medium'>
                                    {t('messageChannels.dialogs.manageBindings.config.allowWithoutMention')}
                                  </Label>
                                  <p className='text-muted-foreground text-xs'>
                                    {t('messageChannels.dialogs.manageBindings.config.allowWithoutMentionHint')}
                                  </p>
                                </div>
                                <Switch
                                  checked={binding.config.allowWithoutMention || false}
                                  onCheckedChange={(checked) => handleConfigChange(index, 'allowWithoutMention', checked)}
                                />
                              </div>
                            )}

                            <div className='space-y-1.5'>
                              <div className='flex items-center justify-between'>
                                <Label className='text-xs font-medium'>
                                  {t('messageChannels.dialogs.manageBindings.config.allowFrom')}
                                </Label>
                                <Button
                                  type='button'
                                  variant='ghost'
                                  size='sm'
                                  onClick={() => handleAddConfigItem(index, 'allowFrom')}
                                  className='h-6 px-2 text-xs'
                                >
                                  <IconPlus className='mr-1 h-3 w-3' />
                                  {t('common.buttons.add')}
                                </Button>
                              </div>
                              {(binding.config.allowFrom || []).length === 0 ? (
                                <p className='text-muted-foreground text-xs italic'>
                                  {t('messageChannels.dialogs.manageBindings.config.allowFromPlaceholder')}
                                </p>
                              ) : (
                                <div className='space-y-1.5'>
                                  {(binding.config.allowFrom || []).map((item, itemIndex) => (
                                    <div key={itemIndex} className='flex items-center gap-2'>
                                      <Input
                                        value={item}
                                        onChange={(e) => handleConfigItemChange(index, 'allowFrom', itemIndex, e.target.value)}
                                        placeholder={t('messageChannels.dialogs.manageBindings.config.allowFromPlaceholder')}
                                        className='h-8 flex-1 text-sm'
                                      />
                                      <Button
                                        type='button'
                                        variant='ghost'
                                        size='icon'
                                        className='text-destructive hover:text-destructive hover:bg-destructive/10 h-8 w-8 shrink-0'
                                        onClick={() => handleRemoveConfigItem(index, 'allowFrom', itemIndex)}
                                      >
                                        <IconTrash className='h-3.5 w-3.5' />
                                      </Button>
                                    </div>
                                  ))}
                                </div>
                              )}
                            </div>

                            <div className='space-y-1.5'>
                              <div className='flex items-center justify-between'>
                                <Label className='text-xs font-medium'>
                                  {t('messageChannels.dialogs.manageBindings.config.excludeKeywords')}
                                </Label>
                                <Button
                                  type='button'
                                  variant='ghost'
                                  size='sm'
                                  onClick={() => handleAddConfigItem(index, 'excludeKeywords')}
                                  className='h-6 px-2 text-xs'
                                >
                                  <IconPlus className='mr-1 h-3 w-3' />
                                  {t('common.buttons.add')}
                                </Button>
                              </div>
                              {(binding.config.excludeKeywords || []).length === 0 ? (
                                <p className='text-muted-foreground text-xs italic'>
                                  {t('messageChannels.dialogs.manageBindings.config.excludeKeywordsPlaceholder')}
                                </p>
                              ) : (
                                <div className='space-y-1.5'>
                                  {(binding.config.excludeKeywords || []).map((item, itemIndex) => (
                                    <div key={itemIndex} className='flex items-center gap-2'>
                                      <Input
                                        value={item}
                                        onChange={(e) => handleConfigItemChange(index, 'excludeKeywords', itemIndex, e.target.value)}
                                        placeholder={t('messageChannels.dialogs.manageBindings.config.excludeKeywordsPlaceholder')}
                                        className='h-8 flex-1 text-sm'
                                      />
                                      <Button
                                        type='button'
                                        variant='ghost'
                                        size='icon'
                                        className='text-destructive hover:text-destructive hover:bg-destructive/10 h-8 w-8 shrink-0'
                                        onClick={() => handleRemoveConfigItem(index, 'excludeKeywords', itemIndex)}
                                      >
                                        <IconTrash className='h-3.5 w-3.5' />
                                      </Button>
                                    </div>
                                  ))}
                                </div>
                              )}
                            </div>
                          </div>
                        </CollapsibleContent>
                      </Collapsible>
                    ))}
                  </div>
                )}
              </ScrollArea>
            </div>
          </div>

          <DialogFooter className='shrink-0 border-t pt-4 sm:gap-2'>
            <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
              {t('common.buttons.cancel')}
            </Button>
            <Button type='button' onClick={handleSave} disabled={saveBindingsMutation.isPending || saveDisabled}>
              {saveBindingsMutation.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={showPairCodeDialog} onOpenChange={(open) => {
        if (!open) {
          onRefresh?.();
        }
        setShowPairCodeDialog(open);
      }}>
        <DialogContent className='sm:max-w-[420px]'>
          <DialogHeader>
            <DialogTitle>{t('messageChannels.dialogs.bindingRequest.pairCode')}</DialogTitle>
            <DialogDescription>{t('messageChannels.dialogs.bindingRequest.pairCodeInstructions')}</DialogDescription>
          </DialogHeader>

          <div className='space-y-4'>
            <div className='text-center'>
              <div className='bg-muted mt-3 rounded-lg p-4'>
                <code className='text-center font-mono text-2xl font-bold tracking-widest'>{generatedPairCode}</code>
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button type='button' onClick={() => {
              onRefresh?.();
              setShowPairCodeDialog(false);
            }}>
              {t('common.buttons.close')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
