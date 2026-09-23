/**
 * The rules a custom placeholder has to satisfy, in one place.
 *
 * A custom placeholder is a name for a template of the built-in ones, kept as a
 * file under the `.mock` folder of the data directory. The backend checks the
 * file's shape (a name it can store, sizes that fit a file), and this module
 * checks the part only the engine can judge: whether the name is one a template
 * can write, and whether the template renders at all.
 *
 * The settings page uses it for the live verdict next to the form and to decide
 * whether Save is allowed; the picker trusts what was saved, so both ends of the
 * feature agree on what a valid placeholder is by construction.
 */
import {
  BUILT_IN_NAMES,
  compileTemplate,
  type CustomPlaceholder,
} from './engine'

/** The shape of a name, identical to what the engine reads after `@`. */
const NAME_PATTERN = /^[A-Za-z_][A-Za-z0-9_]*$/

/** Mirrors of the backend's limits, so the form refuses what a save would. */
export const MAX_NAME_RUNES = 40
export const MAX_TEMPLATE_RUNES = 500
export const MAX_DESCRIPTION_RUNES = 200

/** What the user typed into the placeholder editor. */
export interface CustomDraft {
  name: string
  template: string
  description: string
}

/** One message per field that cannot be saved; a missing field is a good one. */
export interface CustomDraftErrors {
  name?: string
  template?: string
  description?: string
}

/** The bare name: what a user typed, without the `@` they read in a template. */
export function bareName(name: string): string {
  return name.trim().replace(/^@/, '')
}

/**
 * Checks one draft, together with the placeholders it will live beside.
 *
 * `others` is every custom placeholder except the one being edited — a draft may
 * not duplicate another's name, case-insensitively, because the two would be the
 * same file on Windows and macOS. The draft itself is compiled on top of them,
 * so a template that loops back to its own name is reported here rather than in
 * the window.
 */
export function checkCustomPlaceholder(
  draft: CustomDraft,
  others: readonly CustomPlaceholder[],
): CustomDraftErrors {
  const errors: CustomDraftErrors = {}
  const name = bareName(draft.name)
  const template = draft.template.trim()
  const description = draft.description.trim()

  if (name === '') {
    errors.name = 'Give the placeholder a name'
  } else if (!NAME_PATTERN.test(name)) {
    errors.name = 'A name starts with a letter or _ and holds only letters, digits and _'
  } else if (name.length > MAX_NAME_RUNES) {
    errors.name = `At most ${MAX_NAME_RUNES} characters`
  } else if (BUILT_IN_NAMES.has(name)) {
    errors.name = `@${name} is already a built-in placeholder`
  } else {
    const taken = others.find((other) => other.name.toLowerCase() === name.toLowerCase())
    if (taken) errors.name = `@${taken.name} already exists`
  }

  if (template === '') {
    errors.template = 'Write the template this placeholder stands for'
  } else if (template.length > MAX_TEMPLATE_RUNES) {
    errors.template = `At most ${MAX_TEMPLATE_RUNES} characters`
  } else if (!errors.name) {
    // Compiled with the draft in place, so `@me` inside `@me` is caught as the
    // cycle it is.
    const compiled = compileTemplate(template, [...others, { name, template, description }])
    if (compiled.error) errors.template = compiled.error
  }

  if (description.length > MAX_DESCRIPTION_RUNES) {
    errors.description = `At most ${MAX_DESCRIPTION_RUNES} characters`
  }
  return errors
}

/** Whether a set of messages leaves something to save. */
export function isClean(errors: CustomDraftErrors): boolean {
  return !errors.name && !errors.template && !errors.description
}
