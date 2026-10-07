import { Button, Typography } from 'antd';

export function ResourceFilterTags({ label, value, options, onChange }: {
  label: string;
  value?: string;
  options: { value: string; label: string }[];
  onChange: (value?: string) => void;
}) {
  return <div className="resource-filter-row" role="group" aria-label={label}>
    <Typography.Text className="resource-filter-label">{label}</Typography.Text>
    <div className="resource-filter-options">
      {[{ value: '', label: '全部' }, ...options].map(option => <Button key={option.value} size="small"
        type={(value || '') === option.value ? 'primary' : 'text'} aria-pressed={(value || '') === option.value}
        onClick={() => onChange(option.value || undefined)}>{option.label}</Button>)}
    </div>
  </div>;
}
