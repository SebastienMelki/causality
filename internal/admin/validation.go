package admin

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// ValidationError represents a validation error for a specific field.
type ValidationError struct {
	Field   string
	Message string
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

// Add adds a validation error.
func (v *ValidationErrors) Add(field, message string) {
	*v = append(*v, ValidationError{Field: field, Message: message})
}

// HasErrors returns true if there are validation errors.
func (v ValidationErrors) HasErrors() bool {
	return len(v) > 0
}

// GetErrors returns errors for a specific field.
func (v ValidationErrors) GetErrors(field string) []string {
	var errors []string
	for _, e := range v {
		if e.Field == field {
			errors = append(errors, e.Message)
		}
	}
	return errors
}

// HasField returns true if there are errors for a specific field.
func (v ValidationErrors) HasField(field string) bool {
	for _, e := range v {
		if e.Field == field {
			return true
		}
	}
	return false
}

// FormValidator provides form validation utilities.
type FormValidator struct {
	errors ValidationErrors
}

// NewFormValidator creates a new form validator.
func NewFormValidator() *FormValidator {
	return &FormValidator{}
}

// Required validates that a field is not empty.
func (f *FormValidator) Required(field, value, label string) {
	if strings.TrimSpace(value) == "" {
		f.errors.Add(field, fmt.Sprintf("%s is required", label))
	}
}

// MinLength validates minimum string length.
func (f *FormValidator) MinLength(field, value string, min int, label string) {
	if len(value) > 0 && len(value) < min {
		f.errors.Add(field, fmt.Sprintf("%s must be at least %d characters", label, min))
	}
}

// MaxLength validates maximum string length.
func (f *FormValidator) MaxLength(field, value string, max int, label string) {
	if len(value) > max {
		f.errors.Add(field, fmt.Sprintf("%s must be at most %d characters", label, max))
	}
}

// URL validates that a string is a valid URL.
func (f *FormValidator) URL(field, value, label string) {
	if value == "" {
		return
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme == "" || u.Host == "" {
		f.errors.Add(field, fmt.Sprintf("%s must be a valid URL", label))
	}
}

// Email validates that a string is a valid email.
func (f *FormValidator) Email(field, value, label string) {
	if value == "" {
		return
	}
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(value) {
		f.errors.Add(field, fmt.Sprintf("%s must be a valid email address", label))
	}
}

// Int validates that a string is a valid integer.
func (f *FormValidator) Int(field, value, label string) int {
	if value == "" {
		return 0
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		f.errors.Add(field, fmt.Sprintf("%s must be a valid integer", label))
		return 0
	}
	return i
}

// IntRange validates that an integer is within a range.
func (f *FormValidator) IntRange(field string, value, min, max int, label string) {
	if value < min || value > max {
		f.errors.Add(field, fmt.Sprintf("%s must be between %d and %d", label, min, max))
	}
}

// OneOf validates that a value is one of the allowed values.
func (f *FormValidator) OneOf(field, value string, allowed []string, label string) {
	if value == "" {
		return
	}
	for _, a := range allowed {
		if value == a {
			return
		}
	}
	f.errors.Add(field, fmt.Sprintf("%s must be one of: %s", label, strings.Join(allowed, ", ")))
}

// Pattern validates that a string matches a regex pattern.
func (f *FormValidator) Pattern(field, value, pattern, label, message string) {
	if value == "" {
		return
	}
	re := regexp.MustCompile(pattern)
	if !re.MatchString(value) {
		f.errors.Add(field, message)
	}
}

// JSON validates that a string is valid JSON.
func (f *FormValidator) JSON(field, value, label string) {
	if value == "" {
		return
	}
	// Basic JSON validation - check for balanced braces/brackets
	value = strings.TrimSpace(value)
	if len(value) == 0 {
		return
	}
	first := value[0]
	last := value[len(value)-1]
	if (first == '{' && last != '}') || (first == '[' && last != ']') {
		f.errors.Add(field, fmt.Sprintf("%s must be valid JSON", label))
	}
}

// Errors returns all validation errors.
func (f *FormValidator) Errors() ValidationErrors {
	return f.errors
}

// HasErrors returns true if there are validation errors.
func (f *FormValidator) HasErrors() bool {
	return f.errors.HasErrors()
}

// AddError adds a custom validation error.
func (f *FormValidator) AddError(field, message string) {
	f.errors.Add(field, message)
}
