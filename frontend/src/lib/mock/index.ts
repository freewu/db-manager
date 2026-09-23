/**
 * The mock engine, as one entry point.
 *
 * The data generation window only needs these things: the catalogue to show in
 * the picker, a way to compile and render a template, one rendered example of it
 * for a tile or a debug panel, and the two questions that decide what a field
 * holds — what mock it starts with, and how to describe it. The custom
 * placeholders the settings page keeps come in through `CustomPlaceholder`,
 * which is the same shape the window compiles with. Everything else (the word
 * lists, the checksums, the argument parser) is an implementation detail of
 * ./engine.ts.
 */
export { PLACEHOLDER_GROUPS, PLACEHOLDER_NOTES, type Placeholder, type PlaceholderGroup } from './catalog'
export {
  coerceMockValue,
  compileTemplate,
  createRng,
  formatDate,
  sampleOf,
  seedOf,
  BUILT_IN_NAMES,
  type CompiledTemplate,
  type CustomPlaceholder,
  type MockValue,
  type Rng,
  type Sample,
} from './engine'
export {
  bareName,
  checkCustomPlaceholder,
  isClean,
  MAX_DESCRIPTION_RUNES,
  MAX_NAME_RUNES,
  MAX_TEMPLATE_RUNES,
  type CustomDraft,
  type CustomDraftErrors,
} from './custom'
export { defaultMock, mockDescription } from './defaults'
