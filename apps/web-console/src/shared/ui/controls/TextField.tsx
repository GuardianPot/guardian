import { useId, type ChangeEvent, type RefObject } from 'react';
import { Label } from 'radix-ui';
import styles from '@shared/styles/app.module.css';

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
