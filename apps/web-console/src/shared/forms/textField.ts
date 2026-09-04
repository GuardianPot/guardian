/**
 * Reads a text form field.
 *
 * `FormData.get` returns `string | File | null`. Every field in the console is
 * a text input, so a non-string entry cannot occur; reading it as an empty
 * string keeps the value typed without stringifying an object into the
 * request body.
 */
export function textField(form: FormData, name: string): string {
  const value = form.get(name);
  return typeof value === 'string' ? value : '';
}
