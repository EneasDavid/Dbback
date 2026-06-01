package app

import (
	"fmt"
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
	intValue := int64(floatValue)
	if floatValue != float64(intValue) {
		return 0, false
	}
	return intValue, true
}

func matchesUser(row []string, nameIdx int, matriculaIdx int, user SessionUser) bool {
	if nameIdx >= 0 && nameIdx < len(row) && sameLookupValue(row[nameIdx], user.Name, true) {
		return true
	}
	if matriculaIdx >= 0 && matriculaIdx < len(row) && sameLookupValue(row[matriculaIdx], user.Matricula, false) {
		return true
	}
	return false
}

func rowIdentityComment(grid *sheetGrid, rowIdx int) (string, string) {
	if grid == nil || rowIdx < 0 || rowIdx >= len(grid.rowNotes) {
		return "", ""
	}
	for _, colIdx := range identityCommentColumns(grid.headers) {
		if comment := noteAt(grid.rowNotes[rowIdx], colIdx); comment != "" {
			return comment, noteAt(grid.rowNoteAuthors[rowIdx], colIdx)
		}
	}
	return "", ""
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

func excludesStudentFromGrades(comment string) bool {
	normalized := normalizeHeader(comment)
	return strings.Contains(normalized, "nao consta") ||
		strings.Contains(normalized, "nao aparece") ||
		strings.Contains(normalized, "nao encontrado")
}

func studentRow(row []string, nameIdx int, matriculaIdx int) bool {
	if nameIdx >= 0 && strings.TrimSpace(valueAt(row, nameIdx)) != "" {
		return true
	}
	if matriculaIdx >= 0 && strings.TrimSpace(valueAt(row, matriculaIdx)) != "" {
		return true
	}
	return false
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
