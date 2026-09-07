import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
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
  useToasts,
} from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { ENVIRONMENTS_TEXT as TEXT } from './text';

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
    if (!auth.csrf) return setError(TEXT.reauthenticateToCreate);
    const form = event.currentTarget;
    try {
      await create.mutateAsync(textField(new FormData(form), 'display_name'));
      form.reset();
      // A completed action gets a toast; the failure below gets an inline
      // message, never a toast alone (WCX-04 section 9.4).
      toasts.show(TEXT.created);
    } catch {
      setError(TEXT.createFailed);
    }
  }

  const reason = createEnvironment.allowed ? undefined : TEXT.reauthenticateToCreate;

  return (
    <div>
      <header className={styles.pageHeader}>
        <div><p className={styles.eyebrow}>{TEXT.eyebrow}</p><h1>{TEXT.heading}</h1></div>
        <span className={styles.truthNote}>{TEXT.truthNote}</span>
      </header>
      <div className={styles.twoColumn}>
        <Panel
          heading={TEXT.listHeading}
          headingLevel={2}
          aside={<span className={styles.panelCount}>{TEXT.total(environments.data?.length ?? 0)}</span>}
        >
          <DataBoundary
            query={environments}
            subject={{
              name: TEXT.collection,
              dependency: TEXT.dependency,
              stillWorks: TEXT.stillWorks,
              doesNotWork: TEXT.doesNotWork,
              staleReason: TEXT.staleReason,
            }}
            onRetry={() => { void environments.refetch(); }}
            emptyAction={
              createEnvironment.allowed
                ? <Button variant="secondary" onClick={() => nameField.current?.focus()}>{TEXT.nameFirst}</Button>
                : undefined
            }
          >
            {(list) => (
              <ul className={styles.cardList}>
                {list.map((environment) => (
                  <li key={environment.environment_id}>
                    <Link className={styles.environmentCard} to={`/environments/${environment.environment_id}`}>
                      <span><strong>{environment.display_name}</strong><small>{TEXT.zones(environment.zone_count)}</small></span>
                      <StatusBadge encoding={configEncoding(environment.status)} />
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </DataBoundary>
        </Panel>
        <Panel heading={TEXT.createHeading} headingLevel={2} eyebrow={TEXT.createEyebrow}>
          <p>{TEXT.createIntro}</p>
          <form className={styles.form} onSubmit={(event) => { void submit(event); }}>
            <TextField
              name="display_name"
              label={TEXT.displayName}
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
              {TEXT.create}
            </Button>
          </form>
        </Panel>
      </div>
      <ToastRegion toasts={toasts.toasts} onDismiss={toasts.dismiss} />
    </div>
  );
}
