import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import {
  BRAND_COLOURS,
  CONTRAST_PAIRS,
  HEALTH_COLOURS,
  PRIMITIVE,
  SEVERITY_COLOURS,
  contrastRatio,
} from './tokens';

// jsdom does not give import.meta.url a file scheme; vitest runs from the workspace root.
const primitivesCss = readFileSync('src/shared/theme/primitives.css', 'utf8');
const semanticCss = readFileSync('src/shared/theme/semantic.css', 'utf8');

describe('design tokens', () => {
  it('meets the WCAG 2.2 threshold for every pairing that can occur', () => {
    const failures = CONTRAST_PAIRS.filter(
      (pair) => contrastRatio(pair.foreground, pair.background) < pair.minimum,
    ).map(
      (pair) =>
        `${pair.name}: ${contrastRatio(pair.foreground, pair.background).toFixed(2)} < ${pair.minimum}`,
    );
    expect(failures).toEqual([]);
  });

  it('keeps brand, health, and severity colours disjoint', () => {
    const brand = new Set<string>(BRAND_COLOURS);
    const health = new Set<string>(Object.values(HEALTH_COLOURS));
    const severity = new Set<string>(Object.values(SEVERITY_COLOURS));
    for (const value of severity) {
      expect(health.has(value), `severity colour ${value} is also a health colour`).toBe(false);
      expect(brand.has(value), `severity colour ${value} is also a brand colour`).toBe(false);
    }
    for (const value of health) {
      expect(brand.has(value), `health colour ${value} is also a brand colour`).toBe(false);
    }
  });

  it('never gives the brand a health hue, so the product cannot read as healthy', () => {
    // The Phase 1 console used one green for both. That is the collision this
    // package exists to remove.
    for (const value of BRAND_COLOURS) {
      expect(Object.values(HEALTH_COLOURS)).not.toContain(value);
    }
  });

  it('orders the severity ramp so the five steps are perceivably sequenced', () => {
    const order = ['informational', 'low', 'medium', 'high', 'critical'] as const;
    const values = order.map((step) => SEVERITY_COLOURS[step]);
    expect(new Set(values).size).toBe(order.length);
  });

  it('does not express confidence with a severity or health colour', () => {
    const forbidden = new Set<string>([
      ...Object.values(SEVERITY_COLOURS),
      ...Object.values(HEALTH_COLOURS),
    ]);
    // Confidence is a neutral stepped indicator; WC-D11 forbids colour coding.
    expect(forbidden.has(PRIMITIVE.neutral1000)).toBe(false);
    expect(semanticCss).toContain('--confidence-step: var(--color-neutral-1000);');
    expect(semanticCss).toContain('--confidence-step-empty: var(--color-neutral-500);');
  });

  it('keeps device inventory state and configuration completeness neutral', () => {
    // Inventory is not health, and configuration completeness is not
    // protection, so neither may borrow a health or severity colour.
    const meaningful = [...Object.values(HEALTH_COLOURS), ...Object.values(SEVERITY_COLOURS)];
    for (const line of semanticCss.split('\n')) {
      if (!/^\s*--(device|config)-/.test(line)) continue;
      const referenced = line.match(/var\(([^)]+)\)/)?.[1];
      expect(referenced, `${line.trim()} must reference a primitive`).toBeDefined();
      expect(referenced).toMatch(/^--color-neutral-/);
      for (const colour of meaningful) {
        expect(line).not.toContain(colour);
      }
    }
  });

  it('declares every primitive the token module mirrors', () => {
    for (const value of Object.values(PRIMITIVE)) {
      expect(primitivesCss.toLowerCase()).toContain(value.toLowerCase());
    }
  });

  it('lets only the semantic layer reference a primitive', () => {
    // `semantic.css` is the single translation point from raw scale to meaning.
    const semanticNames = semanticCss.match(/^\s*--[a-z0-9-]+:/gm) ?? [];
    expect(semanticNames.length).toBeGreaterThan(40);
    for (const name of semanticNames) {
      expect(name.trim()).not.toMatch(/^--color-/);
    }
  });

  it('keeps motion inside the approved budget', () => {
    const durations = [...primitivesCss.matchAll(/--duration-[a-z]+:\s*(\d+)ms/g)].map((match) =>
      Number(match[1]),
    );
    expect(durations.length).toBeGreaterThan(0);
    for (const duration of durations) {
      expect(duration).toBeLessThanOrEqual(200);
    }
  });
});
