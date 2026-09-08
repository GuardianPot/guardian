import { useId, type ChangeEvent, type FocusEvent, type RefObject } from 'react';
import { Label } from 'radix-ui';
import styles from '@shared/styles/app.module.css';

/**
 * What a form library needs to attach to this input (WCX-11 section 9.1.2).
 *
 * Declared structurally rather than imported from React Hook Form. `@shared/ui`
 * has no business depending on whichever form library the features chose, and a
 * structural type means swapping that library is a change in `@shared/forms`
 * and nowhere else. It happens to be exactly `UseFormRegisterReturn`.
 */
export type FieldRegistration = {
  name: string;
  onChange: (event: ChangeEvent<HTMLInputElement>) => unknown;
  onBlur: (event: FocusEvent<HTMLInputElement>) => unknown;
  ref: (instance: HTMLInputElement | null) => void;
};

/**
 * The single text input (WCX-04 section 9.5).
 *
 * `aria-describedby` wiring is not optional: whichever of description, error,
 * and disabled reason is present is generated, associated, and rendered. A
 * call site cannot forget it, and cannot pass its own `aria-describedby` to
 * replace it.
 *
 * `WCX-11` adds the form library and schema validation; this stays an
 * uncontrolled input read from `FormData`, exactly as the Phase 1 routes do.
 */
export type TextFieldProps = {
  name: string;
  label: string;
  /** Guidance shown before any error. Always associated when present. */
  description?: string;
  /** Field-scoped error. Announced, and marks the control invalid. */
  error?: string;
  /**
   * Id of a form-scoped error that already covers this field, such as a single
   * "sign-in was denied" message across three inputs. The field is marked
   * invalid and described by it, so one message is announced once rather than
   * once per field — but the association is still mandatory, because there is
   * no way to mark a field invalid without naming the message that says why.
   */
  invalidatedBy?: string;
  type?: 'text' | 'password';
  required?: boolean;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  placeholder?: string;
  autoComplete?: string;
  defaultValue?: string;
  /** Controlled value. Supply with `onChange`, or neither for an uncontrolled field. */
  value?: string;
  onChange?: (value: string) => void;
  /** Present only when the field is unavailable. Rendered and associated. */
  disabledReason?: string;
  /**
   * Focus management only. A screen may send focus here — an empty state
   * pointing at the field that creates the first item, for instance. Nothing
   * else may reach into the input through this.
   */
  inputRef?: RefObject<HTMLInputElement | null>;
  /**
   * Binds the control to the form stack. When present it supplies the name,
   * the change and blur handlers, and the ref, so the field is driven by the
   * validator rather than read out of FormData at submit time.
   */
  registration?: FieldRegistration;
};

export function TextField({
  name,
  label,
  description,
  error,
  invalidatedBy,
  type = 'text',
  required,
  minLength,
  maxLength,
  pattern,
  placeholder,
  autoComplete,
  defaultValue,
  value,
  onChange,
  disabledReason,
  inputRef,
  registration,
}: TextFieldProps) {
  const fieldId = useId();
  const invalid = error !== undefined || invalidatedBy !== undefined;
  const describedBy = [
    description === undefined ? '' : `${fieldId}-description`,
    error === undefined ? '' : `${fieldId}-error`,
    invalidatedBy ?? '',
    disabledReason === undefined ? '' : `${fieldId}-reason`,
  ].filter(Boolean).join(' ');
  return (
    <div className={styles.field}>
      <Label.Root htmlFor={fieldId}>{label}</Label.Root>
      {/*
        `registration` is spread last so the form stack owns name, ref, change,
        and blur whenever it is driving this field. Everything above it is a
        default the stack may replace; `id`, `aria-invalid`, and
        `aria-describedby` are not in that set and cannot be overridden.
      */}
      <input
        ref={inputRef}
        id={fieldId}
        name={name}
        type={type}
        disabled={disabledReason !== undefined}
        aria-invalid={invalid ? true : undefined}
        aria-describedby={describedBy === '' ? undefined : describedBy}
        {...(required === undefined ? {} : { required })}
        {...(minLength === undefined ? {} : { minLength })}
        {...(maxLength === undefined ? {} : { maxLength })}
        {...(pattern === undefined ? {} : { pattern })}
        {...(placeholder === undefined ? {} : { placeholder })}
        {...(autoComplete === undefined ? {} : { autoComplete })}
        {...(defaultValue === undefined ? {} : { defaultValue })}
        {...(value === undefined ? {} : { value })}
        {...(onChange === undefined ? {} : { onChange: (event: ChangeEvent<HTMLInputElement>) => onChange(event.target.value) })}
        {...(registration ?? {})}
      />
      {description !== undefined && (
        <span className={styles.fieldDescription} id={`${fieldId}-description`}>{description}</span>
      )}
      {disabledReason !== undefined && (
        <span className={styles.disabledReason} id={`${fieldId}-reason`}>{disabledReason}</span>
      )}
      {error !== undefined && (
        <span className={styles.formError} id={`${fieldId}-error`} role="alert">{error}</span>
      )}
    </div>
  );
}
