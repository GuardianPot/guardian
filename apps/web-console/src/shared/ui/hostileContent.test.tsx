import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { ExerciseEveryComponent } from '@shared/testing/exerciseUi';

/**
 * Hostile content, inline (SEC-08, WCX-04 section 8.1 and 10.1.11).
 *
 * `WCX-06` owns the canonical untrusted-text contract — ANSI escapes, bidi
 * and zero-width characters, attacker-supplied URLs, bounded length. This
 * package asserts the floor that contract will build on: every string a
 * component renders is text, no component renders HTML from data, and nothing
 * a backend or an attacker supplies can create an element or an event handler.
 *
 * The strings below are the ones an attacker actually controls in Phase 1 and
 * Phase 2: a device or environment display name, a health condition reason and
 * message, and the object name typed back into a level 3 confirmation.
 */
const HOSTILE = [
  '<img src=x onerror=alert(1)>',
  '<script>alert(document.cookie)</script>',
  '"><svg onload=alert(1)>',
  "javascript:alert('xss')",
  '<iframe src=//attacker.example></iframe>',
  '{{constructor.constructor("alert(1)")()}}',
  '</strong><style>body{display:none}</style>',
];

/** Elements no component in this layer ever creates; a glyph `svg` is ours. */
const INJECTED = 'img, script, iframe, object, embed, style, link, form, input[type="hidden"]';

describe('hostile content', () => {
  it.each(HOSTILE)('renders %s as inert text and creates no markup', (hostile) => {
    const dialogs: string[] = [];
    const alertSpy = vi.spyOn(window, 'alert').mockImplementation((message?: unknown) => {
      dialogs.push(String(message));
    });
    const { baseElement, unmount } = render(<ExerciseEveryComponent text={hostile} />);

    // Display name, reason, health message, and description all render it.
    expect(baseElement.textContent).toContain(hostile);
    expect(baseElement.querySelectorAll(INJECTED)).toHaveLength(0);
    expect(dialogs).toEqual([]);

    // The angle brackets survive as characters, not as a parsed tag.
    if (hostile.includes('<')) {
      expect(baseElement.innerHTML).toContain('&lt;');
      expect(baseElement.innerHTML).not.toContain(hostile);
    }

    // No element acquired an inline event handler from the string.
    for (const element of baseElement.querySelectorAll('*')) {
      for (const attribute of element.attributes) {
        expect(attribute.name.toLowerCase(), `${element.tagName}[${attribute.name}]`).not.toMatch(/^on/);
      }
    }

    alertSpy.mockRestore();
    unmount();
  });

  it('renders a hostile object name in a level 3 confirmation as text', async () => {
    const hostile = '<img src=x onerror=alert(1)>';
    const { baseElement } = render(<ExerciseEveryComponent text={hostile} />);

    await userEvent.click(screen.getByRole('button', { name: 'Revoke the device' }));
    const dialog = await screen.findByRole('dialog');

    expect(dialog).toHaveTextContent(hostile);
    expect(baseElement.querySelectorAll(INJECTED)).toHaveLength(0);

    // Typing it back enables confirm by exact equality, not by parsing.
    await userEvent.type(screen.getByLabelText('Object name'), hostile);
    expect(screen.getByRole('button', { name: 'Revoke device' })).toBeEnabled();
    expect(baseElement.querySelectorAll(INJECTED)).toHaveLength(0);
  });

  it('renders only the glyphs this layer draws itself', () => {
    const { baseElement } = render(<ExerciseEveryComponent text="<svg onload=alert(1)>" />);
    // Every `svg` present is a status glyph: aria-hidden, no event handlers,
    // and a single `path` child from the encoding table.
    const svgs = [...baseElement.querySelectorAll('svg')];
    expect(svgs.length).toBeGreaterThan(0);
    for (const svg of svgs) {
      expect(svg).toHaveAttribute('aria-hidden', 'true');
      expect(svg.children).toHaveLength(1);
      expect(svg.firstElementChild?.tagName.toLowerCase()).toBe('path');
    }
  });

  it('keeps a hostile string out of every attribute a component writes', () => {
    const hostile = '"><img src=x onerror=alert(1)>';
    const { baseElement } = render(<ExerciseEveryComponent text={hostile} />);

    for (const element of baseElement.querySelectorAll('*')) {
      for (const attribute of element.attributes) {
        // Only `value` may legitimately hold operator- or backend-supplied
        // text, and React sets it as a property, never as parsed markup.
        if (attribute.name === 'value') continue;
        expect(attribute.value, `${element.tagName}[${attribute.name}]`).not.toContain('onerror');
      }
    }
    expect(baseElement.querySelectorAll(INJECTED)).toHaveLength(0);
  });
});
