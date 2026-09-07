import { Link } from 'react-router';
import { Panel } from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { t } from '@shared/text';

/**
 * An address that resolves to nothing (WCX-10 sections 9.7.3 and 8.5).
 *
 * `P1-W11` redirected every unknown path to the environment list. That hides
 * the mistake: an operator who followed a stale link, or mistyped one, lands
 * on a working screen and concludes the link was right. Worse, in a product
 * where the address carries the scope, silently arriving somewhere else is the
 * same class of error as falling back to another environment.
 *
 * So an unknown route says so, keeps the navigation mounted, and offers only
 * **statically known** destinations. Section 8.5: no redirect target is ever
 * read from a query parameter, which is what would make this an open
 * redirect. There is no redirect here at all — these are links the operator
 * chooses.
 *
 * `invalidScope` is a boolean rather than a string discriminant, so the
 * distinction travels without putting a value in a prop — which is the shape
 * the literal-text rule is right to be suspicious of.
 */
export function NotFoundScreen({ invalidScope = false }: { invalidScope?: boolean }) {
  return (
    <div>
      <header className={styles.pageHeader}>
        <div>
          <p className={styles.eyebrow}>{t('notFound.eyebrow')}</p>
          <h1 tabIndex={-1}>{t('notFound.heading')}</h1>
        </div>
      </header>
      <Panel heading={t('notFound.panelHeading')} headingLevel={2}>
        <p>{t(invalidScope ? 'notFound.scopeBody' : 'notFound.routeBody')}</p>
        <nav className={styles.homeLinks} aria-label={t('notFound.entryPoints')}>
          <Link to="/">{t('home.heading')}</Link>
          <Link to="/environments">{t('environments.heading')}</Link>
        </nav>
      </Panel>
    </div>
  );
}

export function NotFoundPage() {
  return <NotFoundScreen />;
}
