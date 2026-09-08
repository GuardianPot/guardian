import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, useState } from 'react';
import { Link } from 'react-router';
import { createEnvironment as createEnvironmentRequest, environmentInvalidation, environmentsQuery } from './api';
import { useAuth, useCapability } from '@features/auth';
import { FormMessage, maxLengthOf, schemaFor, useConsoleForm } from '@shared/forms';
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

const createSchema = schemaFor<'EnvironmentWriteRequest', { display_name: string }>(
  'EnvironmentWriteRequest',
  ['display_name'],
);
const NAME_LIMIT = maxLengthOf('EnvironmentWriteRequest', 'display_name') ?? 512;

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

  /**
   * `WCX-11` moved this onto the form stack. Behaviour is unchanged: the same
   * request, the same toast on success, the same inline message on failure.
   * What changed is where the bound comes from — `maxLength` was hand-copied
   * here and is now derived from the contract, which section 8.2 requires
   * because a copy drifts and a drifted copy accepts what the Control Plane
   * will refuse.
   */
  const form = useConsoleForm<{ display_name: string }>({
    schema: createSchema,
    defaultValues: { display_name: '' },
    onSubmit: async (values) => {
      setError('');
      if (!auth.csrf) {
        setError(t('environments.reauthenticateToCreate'));
        return;
      }
      try {
        await create.mutateAsync(values.display_name);
        form.form.reset({ display_name: '' });
        // A completed action gets a toast; the failure below gets an inline
        // message, never a toast alone (WCX-04 section 9.4).
        toasts.show(t('environments.created'));
      } catch {
        setError(t('environments.createFailed'));
      }
    },
  });

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
          <form className={styles.form} onSubmit={(event) => { void form.submit(event); }}>
            <FormMessage error={form.formError} unattached={form.unattached} id={form.formErrorId} />
            <TextField
              name="display_name"
              label={t('environments.displayName')}
              required
              maxLength={NAME_LIMIT}
              inputRef={nameField}
              registration={form.form.register('display_name')}
              {...(form.form.formState.errors.display_name?.message === undefined
                ? {}
                : { error: String(form.form.formState.errors.display_name.message) })}
              {...(reason === undefined ? {} : { disabledReason: reason })}
            />
            {error && <InlineMessage tone="error">{error}</InlineMessage>}
            <Button
              variant="primary"
              type="submit"
              pending={create.isPending || form.submitting}
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
