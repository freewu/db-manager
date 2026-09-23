// The service side of the custom mock placeholders: the shape a placeholder has
// to have, and the error codes the settings page shows when it does not.
//
// What is *not* checked here is the template's syntax. The engine that reads a
// mock template lives in the window (frontend/src/lib/mock), and it is the only
// implementation of that syntax in this program; a second, poorer parser in Go
// would be a way for the two to disagree about what a template means. So Go
// settles the shape — a name that can be typed as `@name`, a template that is not
// empty, sizes that fit a file per placeholder — and the window, which renders
// the placeholder the moment it is typed, is the one that says whether it works.
package service

import (
	"regexp"
	"strings"

	"dbmanager/internal/apperr"
	"dbmanager/internal/models"
)

const (
	// maxMockNameRunes keeps a name to something a template can hold and a user
	// can read; the file name adds at most a `.json`.
	maxMockNameRunes = 40
	// maxMockTemplateRunes is generous on purpose: a template is a line of text
	// with placeholders in it, and a long one is a literal with a format, not a
	// sign of a mistake.
	maxMockTemplateRunes = 500
	// maxMockDescriptionRunes is a label, not documentation.
	maxMockDescriptionRunes = 200
)

// mockNamePattern is what a placeholder name may be: the same shape the engine
// accepts after `@`, so a name that is stored is a name that can be written.
var mockNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ListMockPlaceholders returns every custom placeholder the user has defined.
func (m *Manager) ListMockPlaceholders() ([]models.MockPlaceholder, error) {
	list, err := m.storeRef().ListMockPlaceholders()
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInternal, err, "read custom mock placeholders")
	}
	return list, nil
}

// SaveMockPlaceholder writes one placeholder, creating it when it does not exist
// yet, and returns what is now on disk.
//
// This is an upsert by name, and the name is the file name: the settings page
// only offers to type a name while creating a placeholder, so a save can never
// move one somewhere the user did not point at.
func (m *Manager) SaveMockPlaceholder(placeholder models.MockPlaceholder) (models.MockPlaceholder, error) {
	clean, err := mockPlaceholderName(placeholder.Name)
	if err != nil {
		return models.MockPlaceholder{}, err
	}
	template := strings.TrimSpace(placeholder.Template)
	if template == "" {
		return models.MockPlaceholder{}, apperr.New(apperr.CodeInvalidConfig,
			"give the placeholder a template — @name alone has nothing to render")
	}
	if len([]rune(template)) > maxMockTemplateRunes {
		return models.MockPlaceholder{}, apperr.New(apperr.CodeInvalidConfig,
			"the template is too long (%d characters max)", maxMockTemplateRunes)
	}
	description := strings.TrimSpace(placeholder.Description)
	if len([]rune(description)) > maxMockDescriptionRunes {
		return models.MockPlaceholder{}, apperr.New(apperr.CodeInvalidConfig,
			"the description is too long (%d characters max)", maxMockDescriptionRunes)
	}

	saved, err := m.storeRef().SaveMockPlaceholder(models.MockPlaceholder{
		Name:        clean,
		Template:    template,
		Description: description,
	})
	if err != nil {
		return models.MockPlaceholder{}, apperr.Wrap(apperr.CodeInternal, err, "write the mock placeholder")
	}
	return saved, nil
}

// DeleteMockPlaceholder removes one placeholder.
func (m *Manager) DeleteMockPlaceholder(name string) error {
	clean, err := mockPlaceholderName(name)
	if err != nil {
		return err
	}
	if err := m.storeRef().DeleteMockPlaceholder(clean); err != nil {
		return apperr.Wrap(apperr.CodeInternal, err, "delete the mock placeholder")
	}
	return nil
}

// mockPlaceholderName trims and checks one name.
//
// The shape is the engine's own (`@` followed by a letter or underscore, then
// letters, digits and underscores), because the name is how a template refers to
// the placeholder: a name that could be stored but not written would be an entry
// the picker offers and no mock can use.
func mockPlaceholderName(name string) (string, error) {
	// A user types the `@` when they read a template, so one is accepted and
	// dropped rather than refused: the stored name is the bare name.
	clean := strings.TrimPrefix(strings.TrimSpace(name), "@")
	if clean == "" {
		return "", apperr.New(apperr.CodeInvalidConfig, "give the placeholder a name")
	}
	if len([]rune(clean)) > maxMockNameRunes {
		return "", apperr.New(apperr.CodeInvalidConfig,
			"the name is too long (%d characters max)", maxMockNameRunes)
	}
	if !mockNamePattern.MatchString(clean) {
		return "", apperr.New(apperr.CodeInvalidConfig,
			"a placeholder name must start with a letter or _ and hold only letters, digits and _: %s", clean)
	}
	return clean, nil
}
