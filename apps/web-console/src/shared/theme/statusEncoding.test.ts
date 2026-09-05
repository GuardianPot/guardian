import { describe, expect, it } from 'vitest';
import {
  CONFIDENCE_ORDER,
  ENCODING_TABLES,
  SEVERITY_ORDER,
  UNKNOWN_ENCODING,
  configEncoding,
  confidenceEncoding,
  deviceEncoding,
  healthEncoding,
  severityEncoding,
} from './statusEncoding';

const INVALID = ['', 'true', 'HEALTHY', 'ok', '<script>', '../../etc', 'unknown'];

describe('status encoding', () => {
  it('gives every value a glyph, a label, and a tone', () => {
    for (const table of Object.values(ENCODING_TABLES)) {
      for (const [value, encoding] of Object.entries(table)) {
        expect(encoding.glyph, `${value} has no glyph`).toBeTruthy();
        expect(encoding.label, `${value} has no label`).toBeTruthy();
        expect(encoding.tone, `${value} has no tone`).toBeTruthy();
      }
    }
  });

  it('distinguishes health values by shape as well as colour', () => {
    const shapes = Object.values(ENCODING_TABLES.HEALTH).map((encoding) => encoding.glyph);
    expect(new Set(shapes).size).toBe(shapes.length);
  });

  it('distinguishes the five severity steps by shape', () => {
    const shapes = SEVERITY_ORDER.map((step) => severityEncoding(step).glyph);
    expect(new Set(shapes).size).toBe(SEVERITY_ORDER.length);
  });

  it('distinguishes the four device states by shape', () => {
    const shapes = Object.values(ENCODING_TABLES.DEVICE).map((encoding) => encoding.glyph);
    expect(new Set(shapes).size).toBe(shapes.length);
  });

  it('resolves an unrecognised value to unknown, never to healthy or complete', () => {
    for (const value of INVALID) {
      for (const lookup of [healthEncoding, severityEncoding, deviceEncoding, configEncoding]) {
        const encoding = lookup(value);
        expect(encoding).toEqual(UNKNOWN_ENCODING);
        expect(encoding.tone).not.toBe('healthTrue');
        expect(encoding.tone).not.toBe('configComplete');
        expect(encoding.tone).not.toBe('severityInformational');
        expect(encoding.label).toBe('Unknown');
      }
    }
  });

  it('never lets a prototype key masquerade as a known status', () => {
    for (const value of ['constructor', '__proto__', 'toString', 'hasOwnProperty']) {
      expect(healthEncoding(value)).toEqual(UNKNOWN_ENCODING);
      expect(deviceEncoding(value)).toEqual(UNKNOWN_ENCODING);
    }
  });

  it('keeps unknown visually distinct from both healthy and failing', () => {
    const healthy = healthEncoding('True');
    const failing = healthEncoding('False');
    const unknown = healthEncoding('Unknown');
    expect(unknown.glyph).not.toBe(healthy.glyph);
    expect(unknown.glyph).not.toBe(failing.glyph);
    expect(unknown.tone).not.toBe(healthy.tone);
    expect(unknown.tone).not.toBe(failing.tone);
  });

  it('expresses confidence as neutral steps, never as a severity tone', () => {
    expect(confidenceEncoding('Low')).toEqual({ filled: 1, total: 3, label: 'Low' });
    expect(confidenceEncoding('Medium')).toEqual({ filled: 2, total: 3, label: 'Medium' });
    expect(confidenceEncoding('High')).toEqual({ filled: 3, total: 3, label: 'High' });
    // No tone at all: WC-D11 forbids colour-coding confidence.
    for (const value of CONFIDENCE_ORDER) {
      expect(confidenceEncoding(value)).not.toHaveProperty('tone');
      expect(confidenceEncoding(value)).not.toHaveProperty('glyph');
    }
  });

  it('reports an unrecognised confidence as unknown with no filled steps', () => {
    for (const value of INVALID) {
      expect(confidenceEncoding(value)).toEqual({ filled: 0, total: 3, label: 'Unknown' });
    }
  });

  it('does not reuse a health tone for a device state', () => {
    // Inventory is not health. Reusing a health tone here would be the exact
    // conflation the product's truth semantics forbid.
    const healthTones = new Set(
      Object.values(ENCODING_TABLES.HEALTH).map((encoding) => encoding.tone),
    );
    for (const encoding of Object.values(ENCODING_TABLES.DEVICE)) {
      expect(healthTones.has(encoding.tone)).toBe(false);
    }
    for (const encoding of Object.values(ENCODING_TABLES.CONFIG)) {
      expect(healthTones.has(encoding.tone)).toBe(false);
    }
  });
});
