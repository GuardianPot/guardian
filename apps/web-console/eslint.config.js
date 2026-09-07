import eslint from '@eslint/js';
import jsxA11y from 'eslint-plugin-jsx-a11y';
import reactHooks from 'eslint-plugin-react-hooks';
import tseslint from 'typescript-eslint';

/**
 * Import boundaries (WCX-01, decision WC-D01).
 *
 *   app/**        may import feature public APIs and shared.
 *   features/a/** may import shared, generated, and its own internals.
 *                 A feature may import another feature only through that
 *                 feature's index, and only for the pairs listed below.
 *   shared/**     may import shared and generated only.
 *   generated/**  imports nothing from the application.
 *
 * Deep imports into another feature are always errors, as are relative
 * imports that escape a feature. Flat config replaces rather than merges a
 * rule's options, so every block below repeats the patterns it needs.
 */
const DEEP_FEATURE_IMPORT = {
  group: ['@features/*/*'],
  message: 'Import another feature through its index (@features/<name>), never a file inside it.',
};

const ESCAPING_RELATIVE_IMPORT = {
  group: ['../*/*', '../../**'],
  message: 'Use an alias (@app, @features, @shared, @generated) instead of a relative import that escapes this directory.',
};

const TEST_ONLY_IMPORT = {
  group: ['@shared/testing', '@shared/testing/**'],
  message: 'Test-only helpers must not be imported by production code.',
};

const NO_APP_IMPORT = {
  group: ['@app/*', '@app/**'],
  message: 'A feature may not import application shell code.',
};

const FEATURES = ['auth', 'environments', 'devices', 'health'];

/** One boundary block per feature. `allowedPeers` documents approved pairs. */
const featureBoundary = (name, allowedPeers) => {
  const forbidden = FEATURES
    .filter((peer) => peer !== name && !allowedPeers.includes(peer))
    .map((peer) => `@features/${peer}`);
  return {
    files: [`src/features/${name}/**/*.{ts,tsx}`],
    ignores: [`src/features/${name}/**/*.test.{ts,tsx}`],
    rules: {
      'no-restricted-imports': ['error', {
        patterns: [
          DEEP_FEATURE_IMPORT,
          ESCAPING_RELATIVE_IMPORT,
          TEST_ONLY_IMPORT,
          NO_APP_IMPORT,
          ...(forbidden.length
            ? [{ group: forbidden, message: `Feature "${name}" has no approved dependency on that feature. Add the pair to eslint.config.js with a reason first.` }]
            : []),
        ],
      }],
    },
  };
};

/**
 * Attributes whose string value is a token, not a sentence (WCX-08 section
 * 9.1). Everything not listed here is treated as operator-facing text and must
 * come from the catalogue, so a new prop is caught by default rather than by
 * someone remembering to add it to a denylist. `data-*` is exempt by prefix.
 *
 * The list is deliberately mechanical: a name is here because the value is a
 * CSS class, an element id, a URL, a form mechanic, an ARIA token from a fixed
 * enumeration, or a design-system variant — never because a particular string
 * looked harmless.
 */
const TECHNICAL_ATTRIBUTES = new Set([
  // identity and structure
  'className', 'id', 'htmlFor', 'key', 'ref', 'slot', 'form', 'role', 'scope',
  'colSpan', 'rowSpan', 'hidden', 'open', 'tabIndex', 'style', 'translate',
  // resources and routing
  'to', 'href', 'src', 'srcSet', 'sizes', 'path', 'action', 'target', 'rel',
  'download', 'loading', 'referrerPolicy', 'crossOrigin', 'integrity',
  // form mechanics
  'type', 'name', 'method', 'encType', 'accept', 'autoComplete', 'inputMode',
  'enterKeyHint', 'pattern', 'step', 'min', 'max', 'maxLength', 'minLength',
  'spellCheck', 'autoCapitalize',
  // ARIA whose value is a token from a closed enumeration, never prose
  'aria-hidden', 'aria-live', 'aria-atomic', 'aria-relevant', 'aria-busy',
  'aria-current', 'aria-modal', 'aria-expanded', 'aria-haspopup',
  'aria-controls', 'aria-labelledby', 'aria-describedby', 'aria-details',
  'aria-owns', 'aria-orientation', 'aria-sort', 'aria-disabled',
  'aria-invalid', 'aria-required', 'aria-pressed', 'aria-selected',
  'aria-checked', 'aria-level', 'aria-errormessage',
  // design-system variants from this repository's own components
  'variant', 'tone', 'precision', 'mode', 'level', 'headingLevel', 'size',
  'align', 'status', 'state', 'encoding', 'appearance',
  // document and SVG plumbing
  'lang', 'dir', 'charSet', 'httpEquiv', 'content', 'dateTime', 'viewBox',
  'xmlns', 'd', 'fill', 'fillRule', 'clipRule', 'stroke', 'strokeWidth',
  'strokeLinecap', 'strokeLinejoin', 'width', 'height', 'x', 'y', 'cx', 'cy',
  'r', 'points', 'preserveAspectRatio', 'focusable',
]);

/**
 * Props that carry a catalogue key rather than a sentence.
 *
 * The `Key` suffix is a convention this rule relies on, and the compiler backs
 * it: every such prop is typed `PlainCatalogueKey`, so a sentence written
 * there does not build. Without the suffix a component that forwards wording
 * to a shared dialog would have to inline the text it is trying not to
 * inline.
 */
const CATALOGUE_KEY_ATTRIBUTE = /Key$/;

/** A run of characters an operator would read as words. */
const READS_AS_WORDS = /\p{L}/u;

/**
 * Forbids operator-facing text written at a call site (WCX-08 section 9.1).
 *
 * A sentence spread across forty components cannot be reviewed as a whole, and
 * wording is this product's differentiator: `SRC-07` provenance phrasing and
 * `EV-04` evidence-before-inference are properties of the words themselves.
 * So the words live in one catalogue and this rule keeps them there.
 *
 * It fires on three shapes, because all three put a sentence in a component:
 * JSX text, a string literal in a child expression, and a string-valued
 * attribute whose name is not a technical one.
 */
const noLiteralText = {
  meta: {
    type: 'problem',
    docs: { description: 'Operator-facing text must come from the @shared/text catalogue.' },
    schema: [],
    messages: {
      child: 'Operator-facing text belongs in the catalogue. Use t() or tx() from @shared/text.',
      attribute: 'Operator-facing "{{name}}" text belongs in the catalogue. Use t() from @shared/text, or add the attribute to TECHNICAL_ATTRIBUTES if its value is a token.',
    },
  },
  create(context) {
    /** The string a literal or expression-free template carries, if any. */
    const literalString = (node) => {
      if (!node) return null;
      if (node.type === 'Literal') return typeof node.value === 'string' ? node.value : null;
      if (node.type === 'TemplateLiteral' && node.expressions.length === 0) {
        return node.quasis.map((quasi) => quasi.value.cooked ?? '').join('');
      }
      return null;
    };

    return {
      JSXText(node) {
        if (READS_AS_WORDS.test(node.value)) context.report({ node, messageId: 'child' });
      },
      JSXExpressionContainer(node) {
        const parent = node.parent?.type;
        if (parent !== 'JSXElement' && parent !== 'JSXFragment') return;
        const value = literalString(node.expression);
        if (value !== null && READS_AS_WORDS.test(value)) {
          context.report({ node, messageId: 'child' });
        }
      },
      JSXAttribute(node) {
        const name = node.name.type === 'JSXIdentifier'
          ? node.name.name
          : `${node.name.namespace.name}:${node.name.name.name}`;
        if (name.startsWith('data-') || TECHNICAL_ATTRIBUTES.has(name)) return;
        if (CATALOGUE_KEY_ATTRIBUTE.test(name)) return;
        const value = node.value?.type === 'JSXExpressionContainer'
          ? literalString(node.value.expression)
          : literalString(node.value);
        if (value !== null && READS_AS_WORDS.test(value)) {
          context.report({ node, messageId: 'attribute', data: { name } });
        }
      },
    };
  },
};

/** Exported so the rule the suite exercises is the rule the build runs. */
export const guardianPlugin = { rules: { 'no-literal-text': noLiteralText } };

export default tseslint.config(
  { ignores: ['dist/**', 'node_modules/**', 'src/generated/**', 'src/**/__boundary__/**'] },
  eslint.configs.recommended,
  ...tseslint.configs.recommendedTypeChecked,
  {
    languageOptions: {
      parserOptions: { projectService: true, tsconfigRootDir: import.meta.dirname },
    },
  },
  { files: ['**/*.{js,mjs}'], ...tseslint.configs.disableTypeChecked },
  {
    files: ['src/**/*.{ts,tsx}'],
    plugins: { 'react-hooks': reactHooks },
    rules: reactHooks.configs.recommended.rules,
  },
  /*
   * Accessibility (WCX-05, decision WC-D23).
   *
   * `WCAG 2.2 Level AA` is the target and this is the first of its three
   * enforcement layers: static rules that fail the build. The second is the
   * component-level axe assertion in `@shared/testing/axe`; the third is the
   * full-page browser scan in `full.yml`.
   *
   * The recommended set runs as errors and keeps its own options — restating a
   * rule with a bare `'error'` would silently replace them. Only the four rules
   * below are changed: three the recommended set does not include, and one
   * whose default is looser than section 9.3 asks for.
   *
   * `no-redundant-roles`, `no-noninteractive-element-interactions`, and
   * `tabindex-no-positive` are section 9.3 requirements that the recommended
   * set already enforces, so they are deliberately not restated here.
   *
   * A suppression must name its reason; `a11ySuppressions.test.ts` fails the
   * suite if one does not.
   */
  {
    files: ['src/**/*.tsx'],
    ...jsxA11y.flatConfigs.recommended,
    rules: {
      ...jsxA11y.flatConfigs.recommended.rules,
      // Section 9.6.1: every input has a programmatically associated label.
      'jsx-a11y/label-has-associated-control': ['error', { assert: 'either' }],
      // Section 9.6.1 again, from the control's side rather than the label's.
      'jsx-a11y/control-has-associated-label': 'error',
      // Section 9.3 prohibits autofocus outright. The recommended default
      // exempts custom components, which is where a screen would hide one.
      'jsx-a11y/no-autofocus': ['error', { ignoreNonDOM: false }],
      // Section 9.2.2 focuses the screen heading, so `tabIndex={-1}` on an
      // `h1` and on `main` is required rather than merely tolerated.
      'jsx-a11y/no-noninteractive-tabindex': ['error', { tags: [], roles: ['tabpanel'], allowExpressionValues: true }],
    },
  },
  /*
   * The text catalogue is the only place operator-facing words are written
   * (WCX-08 section 9.1). Three exemptions, each because the file is fixture
   * material rather than an operator surface:
   *
   * - tests, which have to name the wording they assert on;
   * - the testing harness, including the fixture that proves this rule fires;
   * - `app/workbench`, the development-only component gallery. Its labels name
   *   fixtures ("Denied, no capability"), never product state, and it cannot
   *   reach a production build — `router.tsx` mounts it behind
   *   `import.meta.env.DEV` and `check-bundle.mjs` asserts its marker appears
   *   in no production chunk. Routing fifty gallery captions through the
   *   catalogue would defeat the catalogue's purpose, which is to be readable
   *   in one sitting.
   */
  {
    files: ['src/**/*.tsx'],
    ignores: ['src/**/*.test.tsx', 'src/shared/testing/**', 'src/app/workbench/**'],
    plugins: { guardian: guardianPlugin },
    rules: { 'guardian/no-literal-text': 'error' },
  },
  // Default for production modules outside the layers handled below.
  {
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/**/*.test.{ts,tsx}', 'src/shared/testing/**'],
    rules: {
      'no-restricted-imports': ['error', {
        patterns: [DEEP_FEATURE_IMPORT, ESCAPING_RELATIVE_IMPORT, TEST_ONLY_IMPORT],
      }],
    },
  },
  // shared may never reach upward into features or the application shell.
  {
    files: ['src/shared/**/*.{ts,tsx}'],
    ignores: ['src/shared/testing/**', 'src/shared/**/*.test.{ts,tsx}'],
    rules: {
      'no-restricted-imports': ['error', {
        patterns: [
          { group: ['@features/*', '@features/**', '@app/*', '@app/**'], message: 'shared must not depend on features or the application shell.' },
          ESCAPING_RELATIVE_IMPORT,
          TEST_ONLY_IMPORT,
        ],
      }],
    },
  },
  // Approved cross-feature pairs, each with its reason:
  //   environments -> auth    session state and the capability seam
  //   environments -> health  renders the backend health projection panel
  //   environments -> devices creating an enrollment secret changes the device
  //                           inventory, so the environment feature invalidates it
  //   devices      -> auth    lifecycle actions need the CSRF proof, the
//                           capability seam, and step-up reauthentication
//   devices      -> health  renders the backend health projection panel
  featureBoundary('auth', []),
  featureBoundary('environments', ['auth', 'health', 'devices']),
  featureBoundary('devices', ['auth', 'health']),
  featureBoundary('health', []),
  // Tests may reach the harness but still may not deep-import a feature.
  {
    files: ['src/**/*.test.{ts,tsx}', 'src/shared/testing/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': ['error', { patterns: [DEEP_FEATURE_IMPORT] }],
    },
  },
);
