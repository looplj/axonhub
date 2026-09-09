import { useEffect, useMemo, useRef, useState } from 'react';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { TagsAutocompleteInput } from '@/components/ui/tags-autocomplete-input';
import { useChannels } from '../context/channels-context';
import { useAllChannelTags, useBulkManageChannelTags, useSelectedChannelTags } from '../data/channels';

export function ChannelsBulkManageTagsDialog() {
  const { t } = useTranslation();
  const { open, setOpen, selectedChannels, resetRowSelection, setSelectedChannels } = useChannels();
  const bulkManageChannelTags = useBulkManageChannelTags();
  const { data: allTags = [], isLoading: isLoadingTags } = useAllChannelTags();
  const [tags, setTags] = useState<string[]>([]);

  const isDialogOpen = open === 'bulkManageTags';
  const selectedCount = selectedChannels.length;
  const selectedChannelIDs = useMemo(() => selectedChannels.map((channel) => channel.id).sort(), [selectedChannels]);
  const selectionKey = JSON.stringify(selectedChannelIDs);
  const selectedChannelTagsQuery = useSelectedChannelTags(selectedChannelIDs, { enabled: isDialogOpen });
  const initializedSelectionRef = useRef<string | null>(null);

  const commonTags = useMemo(() => {
    if (!isDialogOpen || selectedChannelIDs.length === 0 || !selectedChannelTagsQuery.data) {
      return [];
    }

    const tagsByChannelID = new Map(selectedChannelTagsQuery.data.map((channel) => [channel.id, new Set(channel.tags)]));
    if (tagsByChannelID.size !== selectedChannelIDs.length) {
      return [];
    }

    const firstChannelTags = tagsByChannelID.get(selectedChannelIDs[0]);
    if (!firstChannelTags) {
      return [];
    }

    return [...firstChannelTags].filter((tag) => selectedChannelIDs.every((id) => tagsByChannelID.get(id)?.has(tag)));
  }, [isDialogOpen, selectedChannelIDs, selectedChannelTagsQuery.data]);

  useEffect(() => {
    if (!isDialogOpen) {
      setTags([]);
      initializedSelectionRef.current = null;
      return;
    }

    if (selectedChannelTagsQuery.isLoading || initializedSelectionRef.current === selectionKey) {
      return;
    }

    setTags(commonTags);
    initializedSelectionRef.current = selectionKey;
  }, [commonTags, isDialogOpen, selectedChannelTagsQuery.isLoading, selectionKey]);

  if (selectedCount === 0 && !isDialogOpen) {
    return null;
  }

  const handleOpenChange = (isOpen: boolean) => {
    if (isOpen) {
      setOpen('bulkManageTags');
      return;
    }

    if (!bulkManageChannelTags.isPending) {
      setTags([]);
      setOpen(null);
    }
  };

  const handleApply = async () => {
    const commonTagSet = new Set(commonTags);
    const addTags = tags.filter((tag) => !commonTagSet.has(tag));
    const removeTags = commonTags.filter((tag) => !tags.includes(tag));

    if (selectedChannelIDs.length === 0 || (addTags.length === 0 && removeTags.length === 0)) {
      return;
    }

    try {
      await bulkManageChannelTags.mutateAsync({ ids: selectedChannelIDs, addTags, removeTags });
      resetRowSelection();
      setSelectedChannels([]);
      setTags([]);
      setOpen(null);
    } catch {
      // Error is already handled by the mutation.
    }
  };

  const hasChanges = tags.some((tag) => !commonTags.includes(tag)) || commonTags.some((tag) => !tags.includes(tag));

  return (
    <Dialog open={isDialogOpen} onOpenChange={handleOpenChange}>
      <DialogContent className='sm:max-w-[600px]'>
        <DialogHeader>
          <DialogTitle>{t('channels.dialogs.bulkManageTags.title')}</DialogTitle>
          <DialogDescription>{t('channels.dialogs.bulkManageTags.description', { count: selectedCount })}</DialogDescription>
        </DialogHeader>

        <div className='space-y-2 py-4'>
          <div className='text-sm font-medium'>{t('channels.dialogs.bulkManageTags.label')}</div>
          {selectedChannelTagsQuery.isLoading ? (
            <div className='text-muted-foreground flex min-h-10 items-center rounded-md border px-3 py-2 text-sm'>
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
              {t('channels.dialogs.bulkManageTags.loading')}
            </div>
          ) : (
            <>
              <TagsAutocompleteInput
                value={tags}
                onChange={setTags}
                placeholder={t('channels.dialogs.bulkManageTags.placeholder')}
                suggestions={allTags}
                isLoading={isLoadingTags}
              />
              {commonTags.length === 0 && (
                <p className='text-muted-foreground text-xs'>{t('channels.dialogs.bulkManageTags.noCommonTags')}</p>
              )}
            </>
          )}
          <p className='text-muted-foreground text-xs'>{t('channels.dialogs.bulkManageTags.hint')}</p>
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => handleOpenChange(false)} disabled={bulkManageChannelTags.isPending}>
            {t('common.buttons.cancel')}
          </Button>
          <Button onClick={handleApply} disabled={!hasChanges || bulkManageChannelTags.isPending || selectedCount === 0}>
            {bulkManageChannelTags.isPending ? (
              <>
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                {t('channels.dialogs.bulkManageTags.applying')}
              </>
            ) : (
              t('channels.dialogs.bulkManageTags.apply')
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
