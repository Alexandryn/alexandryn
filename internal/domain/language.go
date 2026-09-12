package domain

// Language is a validated BCP-47 language tag — a value type with no
// identity of its own, compared by value.
type Language struct {
	tag string
}

// NewLanguage validates tag against validate.go's BCP-47 shape check.
func NewLanguage(tag string) (Language, error) {
	if err := ValidateLanguageTag(tag); err != nil {
		return Language{}, err
	}
	return Language{tag: tag}, nil
}

func (l Language) String() string { return l.tag }
