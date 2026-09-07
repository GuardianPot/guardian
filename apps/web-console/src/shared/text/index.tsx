import { Fragment, type ReactNode } from 'react';
import { CATALOGUE, type CatalogueKey } from './catalogue';

export { CATALOGUE, type CatalogueKey };

/**
 * The typed catalogue accessor (WCX-08 section 9.1).
 *
 * Two guarantees, both from the compiler rather than from review:
 *
 * - an unknown key does not typecheck, so a renamed entry breaks the build
 *   instead of rendering its own key at runtime;
 * - a missing placeholder value does not typecheck. The required values are
 *   derived from the catalogue string itself, so adding `{count}` to an entry
 *   immediately fails every call site that does not supply it.
 *
 * Interpolation produces **text**. `t` returns a string and cannot emit
 * markup; a value containing `<img …>` becomes those characters. Where a
 * sentence genuinely has to wrap a link, `tx` returns nodes — and still never
 * parses anything, because the nodes are supplied by the caller as React
 * elements rather than as a string to be interpreted.
 */
type Entry<K extends CatalogueKey> = (typeof CATALOGUE)[K];

/** Extracts `{name}` placeholders from a catalogue string, as a union. */
type Placeholder<S extends string> = S extends `${string}{${infer Name}}${infer Rest}`
  ? Name | Placeholder<Rest>
  : never;

type Values<K extends CatalogueKey, V> = [Placeholder<Entry<K>>] extends [never]
  ? []
  : [values: Record<Placeholder<Entry<K>>, V>];

const fill = (template: string, values: Record<string, unknown>): string =>
  template.replace(/\{(\w+)\}/g, (whole, name: string) =>
    Object.prototype.hasOwnProperty.call(values, name) ? String(values[name]) : whole,
  );

/** The operator-facing string for a key, with its placeholders filled. */
export function t<K extends CatalogueKey>(key: K, ...args: Values<K, string | number>): string {
  const template: string = CATALOGUE[key];
  const [values] = args;
  return values === undefined ? template : fill(template, values);
}

/**
 * The same, but a placeholder may be a React node.
 *
 * For the handful of sentences that wrap a control — "Read-only session
 * restored. {reauthenticate} before changing configuration." — where splitting
 * the sentence into two catalogue entries would leave neither reviewable and
 * neither translatable.
 */
export function tx<K extends CatalogueKey>(
  key: K,
  ...args: Values<K, ReactNode>
): ReactNode {
  const template: string = CATALOGUE[key];
  const [values] = args;
  if (values === undefined) return template;

  const parts = template.split(/(\{\w+\})/g);
  return parts.map((part, index) => {
    const name = /^\{(\w+)\}$/.exec(part)?.[1];
    const value = name === undefined ? undefined : (values as Record<string, ReactNode>)[name];
    return (
      <Fragment key={index}>{name !== undefined && value !== undefined ? value : part}</Fragment>
    );
  });
}

/**
 * Plural selection through `Intl.PluralRules` (section 9.1.4).
 *
 * A count and a noun are never concatenated at a call site: the caller names a
 * base key and the catalogue holds `.one` and `.other`. English needs only
 * those two, and a language that needs more adds forms to the catalogue rather
 * than logic to a component.
 */
const PLURAL_RULES = new Intl.PluralRules('en');

export type PluralBase = Extract<CatalogueKey, `${string}.one`> extends `${infer Base}.one`
  ? Base
  : never;

export function plural<Base extends PluralBase>(
  base: Base,
  count: number,
  extra: Record<string, string | number> = {},
): string {
  const category = PLURAL_RULES.select(count) === 'one' ? 'one' : 'other';
  const key = `${base}.${category}` as CatalogueKey;
  return fill(CATALOGUE[key], { count, ...extra });
}
