import type { ReactNode } from 'react';
import styles from '../styles/app.module.css';

export function LoadingState({ label = 'Loading current data…' }: { label?: string }) {
  return <p className={styles.state} role="status">{label}</p>;
}

export function ErrorState({ children = 'Guardian could not load this view.' }: { children?: ReactNode }) {
  return <p className={`${styles.state} ${styles.error}`} role="alert">{children}</p>;
}

export function EmptyState({ children }: { children: ReactNode }) {
  return <p className={styles.empty}>{children}</p>;
}
