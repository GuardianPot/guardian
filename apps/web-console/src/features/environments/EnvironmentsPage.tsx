import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { api } from '@shared/api/client';
import { useAuth, useCapability } from '@features/auth';
import { textField } from '@shared/forms/textField';
import { EmptyState, ErrorState, LoadingState } from '@shared/ui/Status';
import styles from '@shared/styles/app.module.css';

export function EnvironmentsPage() {
  const auth = useAuth();
  const queryClient = useQueryClient();
  const [error, setError] = useState('');
  const createEnvironment = useCapability('environment.create');
  const environments = useQuery({ queryKey: ['environments'], queryFn: () => api.environments() });
  const create = useMutation({
    mutationFn: (displayName: string) => api.createEnvironment(displayName, auth.csrf!),
    onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ['environments'] }); },
  });

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError('');
    if (!auth.csrf) return setError('Re-authenticate before creating an environment.');
    const form = event.currentTarget;
    try {
      await create.mutateAsync(textField(new FormData(form), 'display_name'));
      form.reset();
    } catch {
      setError('The environment could not be created. Check the name and try again.');
    }
  }

  return (
    <div>
      <header className={styles.pageHeader}>
        <div><p className={styles.eyebrow}>Organization workspace</p><h1>Environments</h1></div>
        <span className={styles.truthNote}>Configuration is not health</span>
      </header>
      <div className={styles.twoColumn}>
        <section className={styles.panel} aria-labelledby="environment-list-heading">
          <div className={styles.panelHeading}><h2 id="environment-list-heading">Configured environments</h2><span>{environments.data?.length ?? 0} total</span></div>
          {environments.isPending && <LoadingState />}
          {environments.isError && <ErrorState />}
          {environments.data?.length === 0 && <EmptyState>No environments yet. Create the first isolated scope.</EmptyState>}
          <ul className={styles.cardList}>
            {environments.data?.map((environment) => (
              <li key={environment.environment_id}>
                <Link className={styles.environmentCard} to={`/environments/${environment.environment_id}`}>
                  <span><strong>{environment.display_name}</strong><small>{environment.zone_count} {environment.zone_count === 1 ? 'zone' : 'zones'}</small></span>
                  <span className={environment.status === 'zones_defined' ? styles.configReady : styles.configPending}>
                    {environment.status === 'zones_defined' ? 'Configured' : 'Needs zones'}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        </section>
        <section className={styles.panel} aria-labelledby="create-environment-heading">
          <p className={styles.eyebrow}>New boundary</p>
          <h2 id="create-environment-heading">Create environment</h2>
          <p>An environment groups zones and Edge devices. Creation does not scan or alter a network.</p>
          <form className={styles.form} onSubmit={(event) => { void submit(event); }}>
            <label>Display name<input name="display_name" required maxLength={128} disabled={!createEnvironment.allowed || create.isPending} /></label>
            {error && <p className={styles.formError} role="alert">{error}</p>}
            <button className={styles.primaryButton} disabled={!createEnvironment.allowed || create.isPending}>
              {create.isPending ? 'Creating…' : 'Create environment'}
            </button>
          </form>
        </section>
      </div>
    </div>
  );
}
