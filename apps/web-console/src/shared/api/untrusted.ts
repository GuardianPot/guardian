/**
 * The untrusted-value boundary (WCX-06 sections 9.1 and 9.8, decision WC-D24).
 *
 * Guardian is a deception product: a large share of what it displays was
 * written by whoever attacked the network. React escaping stops that content
 * executing, but it does nothing about visual forgery — a right-to-left
 * override that reverses a filename, an ANSI sequence that repaints a
 * transcript, zero-width characters that split a keyword so a search misses
 * it. Escaping is not the same as rendering safely.
 *
 * So untrusted values are not strings here. `Untrusted` is an opaque type: at
 * runtime it is the string it always was, but the compiler will not let it
 * reach JSX, an attribute, a template literal, or anything else that takes a
 * string. The only way to display one is to hand it to `UntrustedText` or
 * `UntrustedBlock`, which apply the contract. That makes the routing a
 * typecheck failure rather than a review comment.
 *
 * `reveal` is the single escape hatch and is deliberately awkward to reach:
 * the renderer needs it, and the clipboard needs it to copy the original
 * value. Nothing else should call it.
 */
declare const untrustedValue: unique symbol;

export type Untrusted<T extends string = string> = {
  readonly [untrustedValue]: T;
};

/**
 * Marks a value as originating outside the trust boundary.
 *
 * Called at the API layer, on every field whose value comes from a device, a
 * decoy, captured telemetry, or an operator-supplied name that a device can
 * influence. It is a cast, not a copy: nothing is transformed here, because
 * transformation is a rendering concern and normalising at the boundary would
 * change the evidence.
 */
export function untrusted<T extends string>(value: T): Untrusted<T> {
  return value as unknown as Untrusted<T>;
}

/** Marks an optional value, keeping `undefined` and `null` distinguishable. */
export function untrustedOrNull(value: string | null | undefined): Untrusted | null {
  return value === null || value === undefined ? null : untrusted(value);
}

/**
 * Reads the original, untransformed value.
 *
 * Only the untrusted renderer and its clipboard action may call this. Every
 * other caller is a routing mistake the opaque type exists to prevent.
 */
export function reveal(value: Untrusted): string {
  return value as unknown as string;
}

/** True when the value carries no characters at all. */
export function isEmptyUntrusted(value: Untrusted | null): boolean {
  return value === null || reveal(value).length === 0;
}
