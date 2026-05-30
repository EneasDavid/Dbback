package app

import (
	"fmt"
	"strconv"
	"strings"
)

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
	return normalizeID(left) == normalizeID(right)
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
	return strings.TrimSpace(notes[idx])
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
