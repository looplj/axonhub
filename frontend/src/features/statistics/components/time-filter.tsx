import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { useTranslation } from 'react-i18next';
import type { StatisticsTimeWindow } from '../data/schema';

interface TimeFilterProps {
  value: StatisticsTimeWindow;
  onChange: (value: StatisticsTimeWindow) => void;
}

export function TimeFilter({ value, onChange }: TimeFilterProps) {
  const { t } = useTranslation();

  return (
    <ToggleGroup type="single" value={value} onValueChange={(v) => v && onChange(v as StatisticsTimeWindow)}>
      <ToggleGroupItem value="day">{t('statistics.daily')}</ToggleGroupItem>
      <ToggleGroupItem value="week">{t('statistics.weekly')}</ToggleGroupItem>
      <ToggleGroupItem value="month">{t('statistics.monthly')}</ToggleGroupItem>
    </ToggleGroup>
  );
}
