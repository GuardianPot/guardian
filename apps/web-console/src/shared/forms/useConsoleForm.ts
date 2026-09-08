import { useCallback, useId, useRef, useState, type BaseSyntheticEvent } from 'react';
import { useForm, type DefaultValues, type FieldValues, type Path } from 'react-hook-form';
import { standardSchemaResolver } from '@hookform/resolvers/standard-schema';
import type { GenericSchema } from 'valibot';
import { toConsoleError, type ConsoleError } from '@shared/api/error';
import { t } from '@shared/text';

/**
 * The console's one form wrapper (WCX-11 section 9.1.2, decision WC-D21).
 *
 * It binds four things every form in this product needs and no screen should
 * re-decide:
 *
 * - the Standard Schema resolver, over a validator derived from the contract;
 * - submission that is always pessimistic (section 9.1.4). No form here applies
 *   an optimistic update, because showing a decoy as renamed before the Control
 *   Plane agreed is the same class of lie as showing it as deployed before an
 *   Edge reported it;
 * - a single in-flight submission (section 9.1.6), so a double click cannot
 *   send a second write;
 * - the mapping from a rejection to where it is shown. With change proposal
 *   0004 the backend names the field, and the message attaches to that control.
 *   Without a field — or with a field this form does not render — it becomes a
 *   form-level error rather than being dropped, because a rejection reason that
 *   disappears leaves an operator with a failure and no cause.
 *
 * Client validation never suppresses a backend rejection (section 8.1). The
 * resolver runs first as a convenience; whatever the Control Plane says
 * afterwards is what the operator is shown.
 */
export type ConsoleFormOptions<Values extends FieldValues> = {
  schema: GenericSchema<Values, Values>;
  defaultValues: DefaultValues<Values>;
  /** Performs the write. Throws to signal rejection; never swallows one. */
  onSubmit: (values: Values) => Promise<void>;
};

export type ConsoleForm<Values extends FieldValues> = ReturnType<typeof useConsoleForm<Values>>;

export function useConsoleForm<Values extends FieldValues>({
  schema,
  defaultValues,
  onSubmit,
}: ConsoleFormOptions<Values>) {
  const form = useForm<Values>({
    resolver: standardSchemaResolver(schema),
    defaultValues,
    // Validate on submit, then keep the field live once it has been corrected.
    // Validating while first typing marks a field invalid before the operator
    // has finished saying what they mean.
    mode: 'onSubmit',
    reValidateMode: 'onChange',
  });

  const [formError, setFormError] = useState<ConsoleError | undefined>(undefined);
  /** Set when the backend named a field this form does not render. */
  const [unattached, setUnattached] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  // A ref rather than the state above: two submits dispatched in the same tick
  // both read the pre-update state, and the second would go out.
  const inFlight = useRef(false);
  const errorId = useId();

  const runSubmit = useCallback(async (values: Values) => {
    if (inFlight.current) return;
    inFlight.current = true;
    setSubmitting(true);
    setFormError(undefined);
    setUnattached(false);
    try {
      await onSubmit(values);
    } catch (caught) {
      const consoleError = toConsoleError(caught);
      let attachedAny = false;
      let missedAny = false;
      for (const fieldError of consoleError.fieldErrors ?? []) {
        // `field` is a backend string. It is compared against this form's own
        // control names and never rendered, so an unrecognised path cannot
        // reach the DOM.
        if (Object.prototype.hasOwnProperty.call(form.getValues(), fieldError.field)) {
          form.setError(fieldError.field as Path<Values>, {
            type: 'server',
            message: t(fieldError.messageKey),
          });
          attachedAny = true;
        } else {
          missedAny = true;
        }
      }
      // The form-level message stands unless every rejection found a home.
      if (missedAny) setUnattached(true);
      if (!attachedAny || missedAny) setFormError(consoleError);
    } finally {
      inFlight.current = false;
      setSubmitting(false);
    }
  }, [form, onSubmit]);

  /**
   * `handleSubmit` is bound at event time rather than during render.
   *
   * Binding it in the render body would hand React Hook Form a function that
   * reads the in-flight ref, which is a ref read during render. Doing it here
   * also means the guard is evaluated against the value at the moment of the
   * click, which is the only moment that matters for a double submission.
   */
  const submit = useCallback(
    (event?: BaseSyntheticEvent) => form.handleSubmit(runSubmit)(event),
    [form, runSubmit],
  );

  /** Clears a stale rejection when the operator starts changing things. */
  const clearFormError = useCallback(() => {
    setFormError(undefined);
    setUnattached(false);
  }, []);

  return {
    form,
    submit,
    submitting,
    /** Present when the failure was not fully attributable to a control. */
    formError,
    /** True when the backend named a field this screen does not render. */
    unattached,
    formErrorId: errorId,
    clearFormError,
    /** Section 9.1.5: the guard reads this; nothing is ever persisted. */
    isDirty: form.formState.isDirty,
  };
}
