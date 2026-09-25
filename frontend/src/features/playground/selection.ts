export type PlaygroundModelSource = 'channel' | 'model_gateway';

export type PlaygroundSelection = {
  modelSource: PlaygroundModelSource;
  selectedChannel: string;
  model: string;
};

export const defaultSelection: PlaygroundSelection = {
  modelSource: 'channel',
  selectedChannel: '',
  model: '',
};

const storageKey = (projectId: string) => `axonhub_playground_selection_${projectId}`;

export function readSelection(projectId: string | null): PlaygroundSelection {
  if (!projectId) return { ...defaultSelection };

  try {
    const stored = localStorage.getItem(storageKey(projectId));
    if (!stored) return { ...defaultSelection };
    const value: unknown = JSON.parse(stored);
    if (
      typeof value !== 'object' ||
      value === null ||
      !('modelSource' in value) ||
      !('selectedChannel' in value) ||
      !('model' in value) ||
      (value.modelSource !== 'channel' && value.modelSource !== 'model_gateway') ||
      typeof value.selectedChannel !== 'string' ||
      typeof value.model !== 'string'
    ) {
      return { ...defaultSelection };
    }
    return {
      modelSource: value.modelSource,
      selectedChannel: value.selectedChannel,
      model: value.model,
    };
  } catch {
    return { ...defaultSelection };
  }
}

export function writeSelection(projectId: string | null, selection: PlaygroundSelection): void {
  if (!projectId) return;
  try {
    localStorage.setItem(storageKey(projectId), JSON.stringify(selection));
  } catch {
    // Storage can be disabled or full; the in-memory selection still works.
  }
}

export function resolveSelection(
  selection: PlaygroundSelection,
  channels: { value: string; models: string[] }[],
  gatewayModels: string[],
  canUseModelGateway: boolean
): PlaygroundSelection {
  const channel = channels.find((item) => item.value === selection.selectedChannel) ?? channels[0];
  const selectedChannel = channel?.value ?? '';
  const modelSource = selection.modelSource === 'model_gateway' && canUseModelGateway ? 'model_gateway' : 'channel';
  const models = modelSource === 'model_gateway' ? gatewayModels : (channel?.models ?? []);
  const model = models.includes(selection.model) ? selection.model : (models[0] ?? '');

  return { modelSource, selectedChannel, model };
}
