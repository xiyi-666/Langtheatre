package contentquality

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var punctuationSpacingRE = regexp.MustCompile(`([A-Za-z0-9])([,;:!?])([A-Za-z])`)

var promptLeakMarkers = []string{
	"task design",
	"create an ielts academic reading drill",
	"create an ielts academic",
	"create an exam-style",
	"no copied official test content",
	"ielts academic stage",
	"learning language code:",
	"json shape:",
	"rules for quiz:",
	"rules for dialogues",
}

var fusedEnglishPhrases = map[string]string{
	"goodmorning":    "Good morning",
	"goodafternoon":  "Good afternoon",
	"goodevening":    "Good evening",
	"goodnight":      "Good night",
	"thankyou":       "Thank you",
	"languagecentre": "Language Centre",
	"languagecenter": "Language Center",
}

var genericReadingQuestions = []string{
	"what is the main focus of the passage?",
	"what is the central argument of the passage?",
	"what is the main idea of the passage?",
	"why are thematic reading units considered effective?",
	"which statement best summarizes the passage?",
}

func NormalizeEnglishSpacing(text string) string {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return ""
	}
	clean = strings.Join(strings.Fields(clean), " ")
	clean = punctuationSpacingRE.ReplaceAllString(clean, "$1$2 $3")
	parts := strings.Fields(clean)
	for i, part := range parts {
		parts[i] = normalizeEnglishToken(part)
	}
	return strings.Join(parts, " ")
}

func HasCollapsedEnglishSpacing(text string) bool {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return false
	}
	if punctuationSpacingRE.MatchString(clean) {
		return true
	}
	for _, token := range strings.Fields(clean) {
		if replacement, ok := fusedEnglishPhrases[strings.ToLower(trimTokenPunctuation(token))]; ok && replacement != "" {
			return true
		}
		if camelTransitionCount(trimTokenPunctuation(token)) >= 2 {
			return true
		}
	}
	return false
}

func ContainsPromptLeak(text string) bool {
	lower := strings.ToLower(text)
	for _, marker := range promptLeakMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func WordCount(text string) int {
	count := 0
	for _, field := range strings.Fields(text) {
		if strings.Trim(field, " \t\r\n.,!?;:\"'()[]{}") != "" {
			count++
		}
	}
	return count
}

func ParagraphCount(text string) int {
	count := 0
	for _, paragraph := range strings.Split(strings.TrimSpace(text), "\n") {
		if strings.TrimSpace(paragraph) != "" {
			count++
		}
	}
	return count
}

func IsGenericReadingQuestion(question string) bool {
	clean := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(question)), " "))
	for _, generic := range genericReadingQuestions {
		if clean == generic {
			return true
		}
	}
	return false
}

func ValidateReadableText(label string, text string, english bool) error {
	if ContainsPromptLeak(text) {
		return fmt.Errorf("%s contains prompt leak", label)
	}
	if english && HasCollapsedEnglishSpacing(text) {
		return fmt.Errorf("%s contains collapsed English spacing", label)
	}
	return nil
}

func normalizeEnglishToken(token string) string {
	leading, core, trailing := splitTokenPunctuation(token)
	if core == "" {
		return token
	}
	if replacement, ok := fusedEnglishPhrases[strings.ToLower(core)]; ok {
		return leading + replacement + trailing
	}
	if camelTransitionCount(core) >= 2 {
		core = splitCamelToken(core)
	}
	return leading + core + trailing
}

func splitCamelToken(token string) string {
	var b strings.Builder
	var prev rune
	for i, r := range token {
		if i > 0 && unicode.IsLower(prev) && unicode.IsUpper(r) {
			b.WriteRune(' ')
		}
		b.WriteRune(r)
		prev = r
	}
	return b.String()
}

func camelTransitionCount(token string) int {
	if token == "" {
		return 0
	}
	count := 0
	var prev rune
	for i, r := range token {
		if i > 0 && unicode.IsLower(prev) && unicode.IsUpper(r) {
			count++
		}
		prev = r
	}
	return count
}

func splitTokenPunctuation(token string) (string, string, string) {
	start := 0
	runes := []rune(token)
	for start < len(runes) && unicode.IsPunct(runes[start]) {
		start++
	}
	end := len(runes)
	for end > start && unicode.IsPunct(runes[end-1]) {
		end--
	}
	return string(runes[:start]), string(runes[start:end]), string(runes[end:])
}

func trimTokenPunctuation(token string) string {
	_, core, _ := splitTokenPunctuation(token)
	return core
}
