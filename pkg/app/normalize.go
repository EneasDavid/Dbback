package app

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var nicknamePatterns = []*regexp.Regexp{
	regexp.MustCompile(`\(([^()]+)\)\s*$`),
	regexp.MustCompile(`"([^"]+)"\s*$`),
	regexp.MustCompile(`'([^']+)'\s*$`),
	regexp.MustCompile(`“([^”]+)”\s*$`),
	regexp.MustCompile(`‘([^’]+)’\s*$`),
}

func normalizeHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("á", "a", "à", "a", "ã", "a", "â", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c")
	return replacer.Replace(value)
}

func normalizeID(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
}

func normalizePerson(value string) string {
	fields := strings.Fields(normalizeHeader(value))
	return strings.Join(fields, " ")
}

func sameLookupValue(left string, right string, person bool) bool {
	if person {
		return normalizePerson(left) == normalizePerson(right)
	}
	if sameNumericID(left, right) {
		return true
	}
	return normalizeID(left) == normalizeID(right)
}

func sameNumericID(left string, right string) bool {
	leftValue, leftOK := parseNumericID(left)
	rightValue, rightOK := parseNumericID(right)
	return leftOK && rightOK && leftValue == rightValue
}

func parseNumericID(value string) (int64, bool) {
	text := strings.TrimSpace(value)
	if text == "" {
		return 0, false
	}
	text = strings.ReplaceAll(text, " ", "")
	text = strings.ReplaceAll(text, ",", ".")
	if parsed, err := strconv.ParseInt(text, 10, 64); err == nil {
		return parsed, true
	}
	floatValue, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, false
	}
	if math.IsNaN(floatValue) || math.IsInf(floatValue, 0) {
		return 0, false
	}
	intValue := int64(floatValue)
	if floatValue != float64(intValue) {
		return 0, false
	}
	return intValue, true
}

func identityCommentColumns(headers []string) []int {
	candidates := []int{nameColumn(headers), groupColumn(headers), matriculaColumn(headers), 0}
	seen := map[int]bool{}
	columns := make([]int, 0, len(candidates))
	for _, colIdx := range candidates {
		if colIdx < 0 || colIdx >= len(headers) || seen[colIdx] {
			continue
		}
		seen[colIdx] = true
		columns = append(columns, colIdx)
	}
	return columns
}

func noteAt(notes []string, idx int) string {
	if idx < 0 || idx >= len(notes) {
		return ""
	}
	return visibleFeedbackComment(notes[idx])
}

func commentAt(notes []string, authors []string, idx int) (string, string) {
	comment := noteAt(notes, idx)
	if comment == "" {
		return "", ""
	}
	return comment, noteAt(authors, idx)
}

func visibleFeedbackComment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	normalized := normalizeHeader(value)
	if strings.Contains(normalized, "montante maximo das atividades") && strings.Contains(normalized, "prova vale") {
		return ""
	}
	return value
}

func authorDisplayName(author string) string {
	author = strings.TrimSpace(author)
	if author == "" {
		return ""
	}
	for _, pattern := range nicknamePatterns {
		match := pattern.FindStringSubmatch(author)
		if len(match) < 2 {
			continue
		}
		nickname := strings.TrimSpace(match[1])
		if validNickname(nickname) {
			return nickname
		}
	}
	return author
}

func validNickname(value string) bool {
	if value == "" || strings.Contains(value, "@") {
		return false
	}
	normalized := normalizeHeader(value)
	return normalized != "ele/dele" &&
		normalized != "ela/dela" &&
		normalized != "they/them" &&
		normalized != "he/him" &&
		normalized != "she/her"
}

func valueAt(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func parseNumber(value string) (float64, bool) {
	text := strings.TrimSpace(strings.ReplaceAll(value, ",", "."))
	if text == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, false
	}
	if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, false
	}
	return parsed, true
}

func formatNumber(value float64) string {
	text := fmt.Sprintf("%.2f", value)
	text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	return strings.ReplaceAll(text, ".", ",")
}

func formatNumberFixed(value float64, precision int) string {
	return strings.ReplaceAll(fmt.Sprintf("%.*f", precision, value), ".", ",")
}
