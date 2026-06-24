import { AppstoreOutlined, BarsOutlined } from '@ant-design/icons';
import { Button, Empty, Segmented, Space, Tag, Tooltip, Typography } from 'antd';
import type { ButtonProps } from 'antd';
import type { ReactNode } from 'react';
import { useEffect, useMemo, useState } from 'react';

export type ListViewMode = 'card' | 'table';

type ListViewToggleProps = {
  storageKey: string;
  value: ListViewMode;
  onChange: (value: ListViewMode) => void;
};

type CardListProps<T> = {
  rows: T[];
  rowKey: (record: T) => string;
  emptyText: string;
  renderCard: (record: T) => ReactNode;
};

export type InfoField = {
  label: string;
  value: ReactNode;
};

type InfoCardProps = {
  title: ReactNode;
  subtitle?: ReactNode;
  status?: ReactNode;
  fields?: InfoField[];
  tags?: ReactNode;
  actions?: ReactNode;
};

export function useListViewMode(storageKey: string, defaultMode: ListViewMode = 'card') {
  const [mode, setMode] = useState<ListViewMode>(() => {
    if (typeof window === 'undefined') return defaultMode;
    return window.localStorage.getItem(storageKey) === 'table' ? 'table' : defaultMode;
  });

  useEffect(() => {
    window.localStorage.setItem(storageKey, mode);
  }, [mode, storageKey]);

  return [mode, setMode] as const;
}

export function ListViewToggle({ storageKey, value, onChange }: ListViewToggleProps) {
  const label = useMemo(() => `列表视图：${storageKey}`, [storageKey]);
  return (
    <div className="view-toggle">
      <Segmented
        aria-label={label}
        value={value}
        onChange={(next) => onChange(next as ListViewMode)}
        options={[
          { label: '卡片', value: 'card', icon: <AppstoreOutlined /> },
          { label: '表格', value: 'table', icon: <BarsOutlined /> }
        ]}
      />
    </div>
  );
}

export function CardList<T>({ rows, rowKey, emptyText, renderCard }: CardListProps<T>) {
  if (rows.length === 0) return <Empty description={emptyText} />;
  return (
    <div className="card-list-grid">
      {rows.map((record) => (
        <div key={rowKey(record)}>{renderCard(record)}</div>
      ))}
    </div>
  );
}

export function InfoCard({ title, subtitle, status, fields = [], tags, actions }: InfoCardProps) {
  return (
    <div className="info-card">
      <div className="info-card-header">
        <div className="info-card-title-group">
          <Typography.Text className="info-card-title">{title}</Typography.Text>
          {subtitle && <Typography.Text type="secondary" className="info-card-subtitle">{subtitle}</Typography.Text>}
        </div>
        {status && <div className="info-card-status">{status}</div>}
      </div>
      {fields.length > 0 && (
        <div className="info-card-fields">
          {fields.map((field) => (
            <div className="info-card-field" key={field.label}>
              <Typography.Text type="secondary">{field.label}</Typography.Text>
              <div>{field.value}</div>
            </div>
          ))}
        </div>
      )}
      {tags && <div className="info-card-tags">{tags}</div>}
      {actions && (
        <div className="info-card-actions">
          <Space wrap>{actions}</Space>
        </div>
      )}
    </div>
  );
}

export function TagGroup({ values, color, emptyText = '无' }: { values?: string[]; color?: string; emptyText?: string }) {
  if (!values || values.length === 0) return <Typography.Text type="secondary">{emptyText}</Typography.Text>;
  return (
    <Space size={[4, 4]} wrap>
      {values.map((value, index) => <Tag key={`${value}-${index}`} color={color}>{value}</Tag>)}
    </Space>
  );
}

type ActionButtonProps = Omit<ButtonProps, 'title'> & {
  tooltip?: ReactNode;
};

export function ActionButton({ children, tooltip, 'aria-label': ariaLabel, className, type = 'text', ...props }: ActionButtonProps) {
  const label = ariaLabel ?? (typeof tooltip === 'string' ? tooltip : undefined) ?? (typeof children === 'string' ? children : undefined);
  const button = (
    <Button
      aria-label={label}
      className={['action-button', !children ? 'action-button-icon' : '', className].filter(Boolean).join(' ')}
      size="small"
      type={type}
      {...props}
    >
      {children}
    </Button>
  );

  if (!tooltip) return button;
  return <Tooltip title={tooltip}>{button}</Tooltip>;
}
