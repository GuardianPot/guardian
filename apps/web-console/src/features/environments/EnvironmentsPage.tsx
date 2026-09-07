import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, useState, type FormEvent } from 'react';
import { Link } from 'react-router';
import { createEnvironment as createEnvironmentRequest, environmentInvalidation, environmentsQuery } from './api';
import { useAuth, useCapability } from '@features/auth';
import { textField } from '@shared/forms/textField';
import { configEncoding } from '@shared/theme/statusEncoding';
import {
  Button,
  DataBoundary,
  InlineMessage,
  Panel,
  StatusBadge,
  TextField,
  ToastRegion,
  UntrustedText,
  useToasts,
} from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { plural, t } from '@shared/text';

export function EnvironmentsPage() {
  const auth = useAuth();
  const queryClient = useQueryClient();
  const [error, setError] = useState('');
  const nameField = useRef<HTMLInputElement>(null);
  const toasts = useToasts();
  const createEnvironment = useCapability('environment.create');
  const environments = useQuery(environmentsQuery());
  const create = useMutation({
    mutationFn: (displayName: string) => createEnvironmentRequest(displayName, auth.csrf!),
    onSuccess: () => environmentInvalidation.afterEnvironmentWrite(queryClient),
  });

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError('');
    if (!auth.csrf) return setError(t('environments.reauthenticateToCreate'));
    const form = event.currentTarget;
    try {
      await create.mutateAsync(textField(new FormData(form), 'display_name'));
      form.reset();
      // A completed action gets a toast; the failure below gets an inline
      // message, never a toast alone (WCX-04 section 9.4).
      toasts.show(t('environments.created'));
    } catch {
      setError(t('environments.createFailed'));
    }
  }

  const reason = createEnvironment.allowed ? undefined : t('environments.reauthenticateToCreate');

  return (
    <div>
      <header className={styles.pageHeader}>
        {/* `tabIndex={-1}` is the route-change focus target (WCX-05 section 9.2.2). */}
        <div><p className={styles.eyebrow}>{t('environments.eyebrow')}</p><h1 tabIndex={-1}>{t('environments.heading')}</h1></div>
        <span className={styles.truthNote}>{t('environments.truthNote')}</span>
      </header>
      <div className={styles.twoColumn}>
        <Panel
          heading={t('environments.listHeading')}
          headingLevel={2}
          aside={<span className={styles.panelCount}>{t('environments.total', { count: environments.data?.length ?? 0 })}</span>}
        >
          <DataBoundary
            query={environments}
            subject={{
              name: t('environments.collection'),
              dependency: t('common.controlPlane'),
              stillWorks: t('environments.stillWorks'),
              doesNotWork: t('environments.doesNotWork'),
              staleReason: t('environments.staleReason'),
            }}
            onRetry={() => { void environments.refetch(); }}
            emptyAction={
              createEnvironment.allowed
                ? <Button variant="secondary" onClick={() => nameField.current?.focus()}>{t('environments.nameFirst')}</Button>
                : undefined
            }
          >
            {(list) => (
              <ul className={styles.cardList}>
                {list.map((environment) => (
                  <li key={environment.environment_id}>
                    <Link className={styles.environmentCard} to={`/environments/${environment.environment_id}`}>
                      <span><strong><UntrustedText value={environment.display_name} /></strong><small>{plural('environments.zoneCount', environment.zone_count)}</small></span>
                      <StatusBadge encoding={configEncoding(environment.status)} />
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </DataBoundary>
        </Panel>
        <Panel heading={t('environments.createHeading')} headingLevel={2} eyebrow={t('environments.createEyebrow')}>
          <p>{t('environments.createIntro')}</p>
          <form className={styles.form} onSubmit={(event) => { void submit(event); }}>
            <TextField
              name="display_name"
              label={t('environments.displayName')}
              required
              maxLength={128}
              inputRef={nameField}
              {...(reason === undefined ? {} : { disabledReason: reason })}
            />
            {error && <InlineMessage tone="error">{error}</InlineMessage>}
            <Button
              variant="primary"
              type="submit"
              pending={create.isPending}
              {...(reason === undefined ? {} : { disabledReason: reason })}
            >
              {t('environments.create')}
            </Button>
          </form>
        </Panel>
      </div>
      <ToastRegion toasts={toasts.toasts} onDismiss={toasts.dismiss} />
    </div>
  );
}
