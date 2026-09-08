import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useParams } from 'react-router';
import { reveal } from '@shared/api/untrusted';
import { toConsoleError } from '@shared/api/error';
import { useAuth, useCapability, useStepUp } from '@features/auth';
import { zonesQuery } from '@features/environments';
import {
  Button,
  ConfirmationDialog,
  DataBoundary,
  InlineMessage,
  Panel,
  confirmationFor,
} from '@shared/ui';
import { t, plural } from '@shared/text';
import styles from '@shared/styles/app.module.css';
import type { DecoyView, DecoyWriteRequest } from '@shared/api/types';
import { createDecoy, decoyInvalidation, decoysQuery, removeDecoy, setDecoyEnabled } from './api';
import { DecoyTable } from './DecoyTable';
import { DecoyForm } from './DecoyForm';

/**
 * Decoy management (WCX-11 sections 9.3, 9.5 and 9.6).
 *
 * Every lifecycle write here is pessimistic. Nothing on this screen changes
 * because the operator asked for it; it changes because the Control Plane
 * confirmed it, and even then what changed is *desired* state. Whether the
 * network followed is the observed record's answer and arrives separately.
 *
 * A failed enable or disable leaves the displayed state exactly where it was
 * and says nothing changed (section 9.9.5). The alternative — optimistically
 * flipping a badge and quietly flipping it back — is how a console ends up
 * disagreeing with the product about what is deployed.
 */
export function DecoysPage() {
  const { environmentId = '' } = useParams();
  const auth = useAuth();
  const client = useQueryClient();
  const stepUp = useStepUp();
  const decoys = useQuery(decoysQuery(environmentId));
  const zones = useQuery(zonesQuery(environmentId));

  const deploy = useCapability(confirmationFor('decoy.deploy').capability);
  const enable = useCapability(confirmationFor('decoy.enable').capability);
  const remove = useCapability(confirmationFor('decoy.remove').capability);

  const [deploying, setDeploying] = useState(false);
  const [removing, setRemoving] = useState<DecoyView | null>(null);
  const [pendingDecoyID, setPendingDecoyID] = useState<string | null>(null);
  const [notice, setNotice] = useState('');
  const [failure, setFailure] = useState('');
  const [conflict, setConflict] = useState(false);

  function beginWrite() {
    setNotice('');
    setFailure('');
    setConflict(false);
  }

  /** Shared tail for every lifecycle write. A conflict is its own state. */
  async function runWrite(
    view: DecoyView,
    write: (csrf: string) => Promise<unknown>,
    success: string,
    failed: string,
  ) {
    beginWrite();
    if (!auth.csrf) {
      setFailure(t('decoys.reauthenticate'));
      return;
    }
    setPendingDecoyID(view.decoy.decoy_id);
    try {
      await write(auth.csrf);
      await decoyInvalidation.afterWrite(client, environmentId);
      setNotice(success);
    } catch (caught) {
      // 409 and 412 both mean the stored decoy is not what this screen was
      // built from. Section 9.9.1 forbids resolving that silently.
      if (toConsoleError(caught).kind === 'conflict') setConflict(true);
      else setFailure(failed);
    } finally {
      setPendingDecoyID(null);
    }
  }

  async function submitDeploy(values: DecoyWriteRequest) {
    beginWrite();
    if (!auth.csrf) throw new Error('missing csrf');
    await createDecoy(environmentId, values, auth.csrf);
    await decoyInvalidation.afterWrite(client, environmentId);
    setDeploying(false);
    setNotice(t('decoys.result.deployed'));
  }

  return (
    <section>
      <h1>{t('decoys.title')}</h1>
      <p>{t('decoys.subtitle')}</p>

      {notice !== '' && <InlineMessage tone="success">{notice}</InlineMessage>}
      {failure !== '' && <InlineMessage tone="error">{failure}</InlineMessage>}
      {conflict && (
        <div className={styles.zoneConflict} role="alert">
          <p>{t('decoys.result.conflict')}</p>
          <Button
            variant="secondary"
            onClick={() => { setConflict(false); void decoyInvalidation.afterWrite(client, environmentId); }}
          >
            {t('decoys.result.reload')}
          </Button>
        </div>
      )}

      <DataBoundary
        query={decoys}
        subject={{
          name: t('decoys.caption'),
          observationSource: t('decoys.observed.neverObserved'),
          dependency: t('common.controlPlane'),
          stillWorks: t('decoys.title'),
          doesNotWork: t('decoys.caption'),
          staleReason: t('decoys.observed.neverObserved'),
        }}
        emptyAction={
          <Button
            variant="primary"
            onClick={() => { beginWrite(); setDeploying(true); }}
            {...(deploy.allowed ? {} : { disabledReason: t('decoys.reauthenticate') })}
          >
            {t('decoys.emptyAction')}
          </Button>
        }
      >
        {(views) => (
          <>
            <p>{plural('decoys.count', views.length)}</p>
            <DecoyTable
              environmentID={environmentId}
              decoys={views}
              pendingDecoyID={pendingDecoyID}
              canEnable={enable.allowed}
              canRemove={remove.allowed}
              disabledReason={t('decoys.reauthenticate')}
              onEnable={(view) => {
                void runWrite(
                  view,
                  (csrf) => setDecoyEnabled(environmentId, view.decoy, true, csrf),
                  t('decoys.result.enabled'),
                  t('decoys.result.enableFailed'),
                );
              }}
              onDisable={(view) => {
                void runWrite(
                  view,
                  (csrf) => setDecoyEnabled(environmentId, view.decoy, false, csrf),
                  t('decoys.result.disabled'),
                  t('decoys.result.disableFailed'),
                );
              }}
              onRemove={(view) => { beginWrite(); setRemoving(view); }}
            />
          </>
        )}
      </DataBoundary>

      {!deploying && (
        <Button
          variant="primary"
          onClick={() => { beginWrite(); setDeploying(true); }}
          {...(deploy.allowed ? {} : { disabledReason: t('decoys.reauthenticate') })}
        >
          {t('decoys.deploy')}
        </Button>
      )}

      {deploying && (
        <Panel heading={t('decoys.deploy')} headingLevel={2}>
          <DecoyForm
            zones={zones.data ?? []}
            defaults={emptyDecoy(zones.data?.[0]?.zone_id ?? '')}
            submitLabel={t('decoys.deploy')}
            onSubmit={submitDeploy}
            onCancel={() => { setDeploying(false); }}
          />
        </Panel>
      )}

      {removing !== null && (
        <ConfirmationDialog
          action="decoy.remove"
          objectName={reveal(removing.decoy.display_name)}
          open
          stepUp={stepUp}
          onCancel={() => { setRemoving(null); }}
          onConfirm={() => {
            const view = removing;
            setRemoving(null);
            void runWrite(
              view,
              (csrf) => removeDecoy(environmentId, view.decoy, csrf),
              t('decoys.result.removed'),
              t('decoys.result.removeFailed'),
            );
          }}
        />
      )}
    </section>
  );
}

/**
 * A blank decoy.
 *
 * The pack fields are left empty rather than pre-filled with the first entry of
 * the index: a default pack is a decision about what an operator is deploying,
 * and the console does not get to make it silently.
 */
function emptyDecoy(zoneID: string): DecoyWriteRequest {
  return {
    zone_id: zoneID,
    display_name: '',
    family: 'ssh',
    persona: 'linux_admin_server',
    address: '',
    pack: '',
    pack_version: '',
  };
}
