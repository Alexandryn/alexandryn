package domain

// Language is a BCP-47 language tag (domain-bibliographic.md FR-5) — a
// value type with no identity of its own, compared by value, safe to
// construct freely once validated.
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
