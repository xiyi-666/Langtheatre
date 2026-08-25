package contentquality

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var punctuationSpacingRE = regexp.MustCompile(`([A-Za-z0-9])([,;:!?])([A-Za-z])`)

var (
	latinLetterRE                    = regexp.MustCompile(`[A-Za-z]`)
	cantoneseBracketedLatinRE        = regexp.MustCompile(`[（(][^（）()\r\n]*[A-Za-z][^（）()\r\n]*[）)]`)
	cantoneseRepeatedCommaRE         = regexp.MustCompile(`[，,、；;]+`)
	cantoneseRepeatedFullStopRE      = regexp.MustCompile(`[。.!！]+`)
	cantoneseRepeatedQuestionRE      = regexp.MustCompile(`[？?]+`)
	cantonesePauseBeforeSentenceRE   = regexp.MustCompile(`，+([。！？])`)
	cantoneseWhitespacePunctuationRE = regexp.MustCompile(`\s*([，。！？：])\s*`)
	cantoneseDrinkModifierWithCupRE  = regexp.MustCompile(`一杯((?:凍|冻|熱|热)?(?:奶茶|檸茶|柠茶|咖啡|鴛鴦|鸳鸯|檸水|柠水|朱古力|利賓納|利宾纳|可樂|可乐))(?:要)?(少甜|走甜|少冰|走冰)([，。！？]|$)`)
	cantoneseDrinkModifierRE         = regexp.MustCompile(`((?:凍|冻|熱|热)?(?:奶茶|檸茶|柠茶|咖啡|鴛鴦|鸳鸯|檸水|柠水|朱古力|利賓納|利宾纳|可樂|可乐))(?:要)?(少甜|走甜|少冰|走冰)([，。！？]|$)`)
)

var cantoneseLatinReplacements = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)\bQR\s*code\b`), "二維碼"},
	{regexp.MustCompile(`(?i)\bWi[\s-]*Fi\b`), "無線網絡"},
	{regexp.MustCompile(`(?i)\bWhatsApp\b`), "即時通訊軟件"},
	{regexp.MustCompile(`(?i)\be-?mail\b`), "電郵"},
	{regexp.MustCompile(`(?i)\bMTR\b`), "港鐵"},
	{regexp.MustCompile(`(?i)\bATM\b`), "提款機"},
	{regexp.MustCompile(`(?i)\bVIP\b`), "貴賓"},
	{regexp.MustCompile(`(?i)\bPDF\b`), "文件"},
	{regexp.MustCompile(`(?i)\bAPP\b`), "應用程式"},
	{regexp.MustCompile(`(?i)\bonline\b`), "網上"},
	{regexp.MustCompile(`(?i)\bAI\b`), "人工智能"},
	{regexp.MustCompile(`(?i)\bID\b`), "身份證明"},
	{regexp.MustCompile(`(?i)\bUber\b`), "網約車"},
	{regexp.MustCompile(`(?i)\bZoom\b`), "視像會議"},
	{regexp.MustCompile(`(?i)\bOK\b`), "好"},
}

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
	"welcome to today's mini-theater",
	"welcome to mini theater",
	"today's topic is",
	"our topic is [ielts",
	"[ielts listening]",
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

// NormalizeSpeakerLabel removes presentation-only Markdown so the same
// character keeps one stable identity across validation, storage and TTS.
func NormalizeSpeakerLabel(label string) string {
	clean := strings.TrimSpace(label)
	if clean == "" {
		return ""
	}
	clean = strings.NewReplacer(
		"**", "",
		"__", "",
		"`", "",
	).Replace(clean)
	return strings.TrimSpace(strings.Trim(clean, "*_#"))
}

// NormalizeCantoneseSpeechText removes formatting that commonly causes long
// TTS pauses and converts frequent English interface terms into natural Hong
// Kong Chinese. It deliberately keeps unknown names intact; generation
// validation is responsible for rejecting new Cantonese lines with Latin text.
func NormalizeCantoneseSpeechText(text string) string {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return ""
	}
	for _, replacement := range cantoneseLatinReplacements {
		clean = replacement.pattern.ReplaceAllString(clean, replacement.replacement)
	}
	clean = cantoneseBracketedLatinRE.ReplaceAllString(clean, "")
	clean = strings.NewReplacer(
		"\r\n", "，",
		"\n", "，",
		"\r", "，",
		"……", "，",
		"...", "，",
		"…", "，",
		"——", "，",
		"—", "，",
		"；", "，",
		";", "，",
		"（", "，",
		"）", "，",
		"(", "，",
		")", "，",
	).Replace(clean)
	clean = cantoneseRepeatedCommaRE.ReplaceAllString(clean, "，")
	clean = cantoneseRepeatedFullStopRE.ReplaceAllString(clean, "。")
	clean = cantoneseRepeatedQuestionRE.ReplaceAllString(clean, "？")
	clean = cantonesePauseBeforeSentenceRE.ReplaceAllString(clean, "$1")
	clean = cantoneseWhitespacePunctuationRE.ReplaceAllString(clean, "$1")
	// Menu-style keyword stacks such as "凍奶茶少甜" often make TTS read
	// syllable by syllable. Rewrite them as a complete Cantonese noun phrase,
	// which gives the synthesizer a natural prosodic unit.
	clean = cantoneseDrinkModifierWithCupRE.ReplaceAllString(clean, "一杯${2}嘅${1}${3}")
	clean = cantoneseDrinkModifierRE.ReplaceAllString(clean, "一杯${2}嘅${1}${3}")
	clean = strings.Trim(clean, " ，,")
	if !ContainsLatinLetters(clean) {
		clean = strings.Join(strings.Fields(clean), "")
	} else {
		clean = strings.Join(strings.Fields(clean), " ")
	}
	return clean
}

func ContainsLatinLetters(text string) bool {
	return latinLetterRE.MatchString(text)
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
