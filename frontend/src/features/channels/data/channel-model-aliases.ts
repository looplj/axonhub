import type { ChannelModelEntry } from './schema';

export interface ChannelModelGroup {
  model: string;
  originalVisible: boolean;
  aliases: ChannelModelEntry[];
}

// Keep upstream models as the anchors, even when their original request names
// are hidden. Only backend-resolved names are shown as usable aliases.
export function groupChannelModelEntries(models: string[], entries: ChannelModelEntry[]): ChannelModelGroup[] {
  const groups = new Map<string, ChannelModelGroup>();
  for (const model of models) {
    groups.set(model, { model, originalVisible: false, aliases: [] });
  }
  for (const entry of entries) {
    const group = groups.get(entry.actualModel);
    if (!group) continue;
    if (entry.requestModel === entry.actualModel) {
      group.originalVisible = true;
    } else {
      group.aliases.push(entry);
    }
  }
  return [...groups.values()];
}

export function filterChannelModelGroups(groups: ChannelModelGroup[], search: string): ChannelModelGroup[] {
  const query = search.trim().toLowerCase();
  return groups.filter(
    (group) => group.model.toLowerCase().includes(query) || group.aliases.some((alias) => alias.requestModel.toLowerCase().includes(query))
  );
}
