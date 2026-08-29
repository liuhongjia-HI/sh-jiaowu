import { Button, Drawer, Space } from 'antd';
import type { DrawerProps } from 'antd';
import type { ReactNode } from 'react';

type FormDrawerProps = Omit<DrawerProps, 'footer' | 'onClose'> & {
  cancelText?: ReactNode;
  onCancel: () => void;
  onSubmit: () => void;
  submitDisabled?: boolean;
  submitText?: ReactNode;
  submitting?: boolean;
};

export function FormDrawer({
  cancelText = '取消',
  children,
  destroyOnHidden = true,
  onCancel,
  onSubmit,
  submitDisabled = false,
  submitText = '保存',
  submitting = false,
  width = 'min(560px, 100vw)',
  ...drawerProps
}: FormDrawerProps) {
  return (
    <Drawer
      {...drawerProps}
      destroyOnHidden={destroyOnHidden}
      footer={(
        <div style={{ textAlign: 'right' }}>
          <Space>
            <Button onClick={onCancel}>{cancelText}</Button>
            <Button type="primary" disabled={submitDisabled} loading={submitting} onClick={onSubmit}>
              {submitText}
            </Button>
          </Space>
        </div>
      )}
      onClose={onCancel}
      width={width}
    >
      {children}
    </Drawer>
  );
}
