import * as v from 'valibot';
import { CONSTRAINTS } from '@generated/constraints';
import { t, type PlainCatalogueKey } from '@shared/text';

/**
 * Validators derived from the contract (WCX-11 sections 8.2 and 9.1.3).
 *
 * Nothing here writes a length, a range, or a pattern. Every one is read from
 * `@generated/constraints`, which is generated from `openapi/guardian.yaml` and
 * checked for staleness by `generated:check`. Hand-copying a constraint is
 * forbidden by section 8.2 because a drifted copy silently accepts input the
 * Control Plane will reject, and the operator then sees a save fail for a
 * reason the form said was fine.
 *
 * Client validation is a convenience and nothing more (section 8.1). It exists
 * to catch a typo before a round trip. The Control Plane remains authoritative,
 * and `useConsoleForm` renders a backend rejection truthfully even when this
 * layer believed the value was good.
 */

type SchemaName = keyof typeof CONSTRAINTS;

/** The keyword subset the generator emits. Widened from the frozen literals. */
type Constraint = {
  type?: string;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  format?: string;
  enum?: readonly string[];
};

/**
 * The message a failed constraint carries.
 *
 * Deliberately generic per keyword rather than per field. A message naming the
 * exact bound would be a second copy of the contract in the text catalogue,
 * drifting the moment the contract moves — the same failure section 8.2 is
 * about, one layer up.
 */
const MESSAGE: Record<'required' | 'tooShort' | 'tooLong' | 'pattern' | 'choice', PlainCatalogueKey> = {
  required: 'forms.field.required',
  tooShort: 'forms.field.tooShort',
  tooLong: 'forms.field.tooLong',
  pattern: 'forms.field.pattern',
  choice: 'forms.field.choice',
};

/** Builds one field validator from its generated constraint. */
function fieldSchema(constraint: Constraint, required: boolean) {
  if (constraint.enum !== undefined && constraint.enum.length > 0) {
    // A closed vocabulary. `picklist` rejects anything outside it, which is
    // also what makes a tampered <select> option fail here rather than at the
    // Control Plane.
    return v.picklist(constraint.enum as unknown as [string, ...string[]], t(MESSAGE.choice));
  }
  const checks: v.GenericPipeAction<string, string, v.BaseIssue<unknown>>[] = [];
  // An empty required field reads as "required", not as "too short". They are
  // the same check to a validator and different sentences to an operator.
  if (required) checks.push(v.minLength(1, t(MESSAGE.required)));
  if (constraint.minLength !== undefined && constraint.minLength > 1) {
    checks.push(v.minLength(constraint.minLength, t(MESSAGE.tooShort)));
  }
  if (constraint.maxLength !== undefined) {
    checks.push(v.maxLength(constraint.maxLength, t(MESSAGE.tooLong)));
  }
  if (constraint.pattern !== undefined) {
    checks.push(v.regex(new RegExp(constraint.pattern, 'u'), t(MESSAGE.pattern)));
  }
  return v.pipe(v.string(), ...checks);
}

/**
 * A form field that may satisfy any one of several contract fields.
 *
 * Sign-in has exactly one: a single `proof` control carries either a TOTP code
 * or a recovery code, because the operator picks the method and the contract's
 * `oneOf` says the request carries one or the other. Validating it against both
 * shapes keeps the bound derived from the contract rather than retyped, which
 * is the whole point of section 8.2 — the alternative was a hand-written
 * `[0-9]{6}` in a component, drifting the moment the contract changed.
 */
export type Alternatives<Name extends SchemaName> = Readonly<
  Record<string, readonly (keyof (typeof CONSTRAINTS)[Name]['fields'] & string)[]>
>;

/**
 * Builds the validator for one contract write request.
 *
 * `fields` narrows to the subset a given form actually renders — the decoy
 * update form omits nothing, but a screen that edits one field should not be
 * made to submit seven.
 */
export function schemaFor<
  Name extends SchemaName,
  Values extends Record<string, string> = Record<string, string>,
>(
  name: Name,
  fields: readonly (keyof (typeof CONSTRAINTS)[Name]['fields'] & string)[],
  alternatives: Alternatives<Name> = {},
): v.GenericSchema<Values, Values> {
  const declaration = CONSTRAINTS[name];
  const required: readonly string[] = declaration.required;
  const source = declaration.fields as Record<string, Constraint>;
  const entries: Record<string, v.GenericSchema<string, string>> = {};
  for (const field of fields) {
    const constraint = source[field];
    if (constraint === undefined) {
      // Unreachable through the type parameter; kept because a regenerated
      // contract that drops a field should fail loudly rather than validate
      // nothing at all.
      throw new Error(`contract schema ${name} has no field ${field}`);
    }
    entries[field] = fieldSchema(constraint, required.includes(field));
  }
  for (const [formField, contractFields] of Object.entries(alternatives)) {
    const options = contractFields.map((contractField) => {
      const constraint = source[contractField];
      if (constraint === undefined) {
        throw new Error(`contract schema ${name} has no field ${contractField}`);
      }
      // Required against every alternative: the control itself is required, and
      // which contract field it satisfies is the operator's method choice.
      return fieldSchema(constraint, true);
    });
    entries[formField] = v.union(options, t(MESSAGE.pattern));
  }
  /*
   * The one cast in this module, and it is narrowing a runtime-built schema to
   * the shape the caller says it expects.
   *
   * The schema is assembled from generated constraints, so its key set is not
   * a literal the compiler can see. What keeps the two honest is not this line:
   * it is that `fields` is key-checked against the generated declaration above,
   * and that `generated:check` fails if the declaration drifts from the
   * contract. A caller naming a `Values` shape the contract does not have would
   * fail at the `fields` argument first.
   */
  return v.object(entries) as unknown as v.GenericSchema<Values, Values>;
}

/**
 * The contract's own bound for a field, for a control's `maxLength`.
 *
 * The browser's native truncation is a nicety, not a control: it is read from
 * the same generated source so it cannot disagree with the validator.
 */
export function maxLengthOf<Name extends SchemaName>(
  name: Name,
  field: keyof (typeof CONSTRAINTS)[Name]['fields'] & string,
): number | undefined {
  const constraint = (CONSTRAINTS[name].fields as Record<string, Constraint>)[field];
  return constraint?.maxLength;
}

/** The closed option set for an enumerated field, for a `<select>`. */
export function choicesOf<Name extends SchemaName>(
  name: Name,
  field: keyof (typeof CONSTRAINTS)[Name]['fields'] & string,
): readonly string[] {
  const constraint = (CONSTRAINTS[name].fields as Record<string, Constraint>)[field];
  return constraint?.enum ?? [];
}
