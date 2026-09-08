/**
 * The form and validation stack (WCX-11 section 9.1, decision WC-D21).
 *
 * React Hook Form with a Standard Schema validator, over schemas derived from
 * the contract rather than hand-copied from it. A screen composes from here and
 * never reaches for the library directly, so the pessimistic-submit,
 * single-submission, and field-error rules hold everywhere by construction.
 *
 * The `textField` FormData reader this package replaced is gone: every form in
 * the console is driven by the stack now, so there is nothing left to read a
 * value out of a submit event.
 */
export { schemaFor, maxLengthOf, choicesOf } from './schema';
export { useConsoleForm, type ConsoleForm, type ConsoleFormOptions } from './useConsoleForm';
export { UnsavedChangesGuard } from './UnsavedChangesGuard';
export { FormMessage } from './FormMessage';
