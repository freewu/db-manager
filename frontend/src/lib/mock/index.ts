/**
 * The mock engine, as one entry point.
 *
 * The data generation window only needs these four things: the catalogue to
 * show in the picker, a way to compile and render a template, and the two
 * questions that decide what a field holds — what mock it starts with, and how
 * to describe it. Everything else (the word lists, the checksums, the argument
 * parser) is an implementation detail of ./engine.ts.
 */
export { PLACEHOLDER_GROUPS, PLACEHOLDER_NOTES, type Placeholder, type PlaceholderGroup } from './catalog'
export {
  coerceMockValue,
  compileTemplate,
  createRng,
  formatDate,
  type CompiledTemplate,
  type MockValue,
  type Rng,
} from './engine'
export { defaultMock, mockDescription } from './defaults'
