package app

import (
	"math"
	"strings"
	"unicode"
)

const officialQuestionRubricMaximum = 10

var inferredCriterionMaxima = map[string]float64{
	"adequacao":    1,
	"organizacao":  0.5,
	"q.1":          1,
	"q.2":          1.5,
	"q.3":          1.5,
	"q.4":          2,
	"q.5":          1,
	"q.6":          1.5,
	"semana 1":     0.25,
	"semana 2":     0.25,
	"semana 3":     0.25,
	"semana 4":     0.25,
	"sgbd":         1,
	"dataset":      1,
	"crud":         1,
	"apresentacao": 2,
}

type activityItem struct {
	Key           string
	Subtopic      string
	NotaMaxima    string
	NotaAlcancada string
	Comment       string
	CommentAuthor string
}

type studentCell struct {
	Key           string
	Header        string
	Label         string
	Value         string
	Comment       string
	CommentAuthor string
}

func makeCard(key string, label string, value string, comment string, commentAuthor string, details []DetailResult) CardResult {
	return CardResult{
		Key:           key,
		Label:         label,
		Value:         value,
		DisplayValue:  displayValue(label, value),
		Tone:          scoreTone(label, value),
		Comment:       comment,
		CommentAuthor: commentAuthor,
		Details:       details,
	}
}

func activityDetails(items []activityItem) []DetailResult {
	details := make([]DetailResult, 0, len(items))
	for _, item := range items {
		if normalizeHeader(item.Subtopic) == "total" {
			continue
		}
		maximum, _ := parseScore(item.NotaMaxima)
		detail := scoreDetail(item.Key, strings.TrimSpace(item.Subtopic), item.NotaAlcancada, maximum)
		detail.Comment = item.Comment
		detail.CommentAuthor = item.CommentAuthor
		details = append(details, detail)
	}
	return details
}

func activityItemsForWeight(items []activityItem, weight float64) []activityItem {
	if weight <= 0 {
		return items
	}
	totalMaximum := 0.0
	for _, item := range items {
		if normalizeHeader(item.Subtopic) == "total" {
			continue
		}
		if maximum, ok := parseScore(item.NotaMaxima); ok && maximum > 0 {
			totalMaximum += maximum
		}
	}

	normalized := append([]activityItem(nil), items...)
	for idx, item := range normalized {
		maximum, ok := parseScore(item.NotaMaxima)
		if !ok || maximum <= 0 {
			continue
		}
		if normalizeHeader(item.Subtopic) == "total" {
			normalized[idx].NotaAlcancada = normalizedScore(item.NotaAlcancada, maximum, weight)
			normalized[idx].NotaMaxima = formatNumber(weight)
			continue
		}
		if totalMaximum <= 0 {
			continue
		}
		value := normalizedScore(item.NotaAlcancada, maximum, maximum)
		normalized[idx].NotaAlcancada = normalizedScore(value, totalMaximum, weight)
		normalized[idx].NotaMaxima = formatNumber(normalizedMaximum(maximum, totalMaximum, weight))
	}
	return normalized
}

func activityItemsForDivisor(items []activityItem, divisor float64) []activityItem {
	if divisor <= 1 {
		return items
	}
	normalized := append([]activityItem(nil), items...)
	for idx, item := range normalized {
		maximum, ok := parseScore(item.NotaMaxima)
		if !ok || maximum <= 0 {
			continue
		}
		normalized[idx].NotaAlcancada = normalizedScore(item.NotaAlcancada, maximum, maximum/divisor)
		normalized[idx].NotaMaxima = formatNumber(maximum / divisor)
	}
	return normalized
}

func columnDetails(cells []studentCell) []DetailResult {
	return cellDetails(cells, isDetailOnlyColumn)
}

func projectDetails(cells []studentCell) []DetailResult {
	return cellDetails(cells, projectDetailColumn)
}

func cellDetails(cells []studentCell, include func(string) bool) []DetailResult {
	details := make([]DetailResult, 0, len(cells))
	for _, cell := range cells {
		if !include(cell.Header) {
			continue
		}
		maximum := inferMaxForLabel(cell.Header)
		if maximum <= 0 {
			maximum = 1
		}
		detail := scoreDetail(cell.Key, cell.Label, cell.Value, maximum)
		detail.Comment = cell.Comment
		detail.CommentAuthor = cell.CommentAuthor
		details = append(details, detail)
	}
	return details
}

func scoreDetail(key string, label string, value string, maximum float64) DetailResult {
	obtained, hasObtained := parseScore(value)
	pending := strings.TrimSpace(value) == ""
	ratio := 0.0
	if !pending && hasObtained && maximum > 0 {
		ratio = math.Min(math.Max((obtained/maximum)*100, 0), 100)
	}
	return DetailResult{
		Key:          key,
		Label:        label,
		Value:        value,
		Max:          maximum,
		DisplayScore: detailDisplayScore(value, maximum, pending),
		Ratio:        ratio,
		Pending:      pending,
		Tone:         scoreToneFromRatio(ratio, pending),
	}
}

func percentageActivityDetails(items []activityItem) []DetailResult {
	details := activityDetails(items)
	for idx := range details {
		details[idx].Percentage = true
		if !details[idx].Pending {
			details[idx].DisplayScore = formatNumber(details[idx].Ratio) + "%"
		}
	}
	return details
}

func normalizedScore(value string, sourceMaximum float64, targetMaximum float64) string {
	score, ok := parseScore(value)
	if !ok || sourceMaximum <= 0 || targetMaximum <= 0 {
		return strings.TrimSpace(value)
	}
	if score > sourceMaximum {
		score = sourceMaximum
	}
	return formatScore((score / sourceMaximum) * targetMaximum)
}

func normalizedMaximum(sourceMaximum float64, totalMaximum float64, targetMaximum float64) float64 {
	if sourceMaximum <= 0 || totalMaximum <= 0 || targetMaximum <= 0 {
		return sourceMaximum
	}
	return (sourceMaximum / totalMaximum) * targetMaximum
}

func detailDisplayScore(value string, maximum float64, pending bool) string {
	if pending {
		return "Em correção"
	}
	if obtained, ok := parseScore(value); ok && maximum > 0 {
		return scoreComparisonDisplay(obtained, maximum)
	}
	if maximum > 0 {
		return "Max " + formatGradeNumber(maximum)
	}
	return strings.TrimSpace(value)
}

func displayValue(label string, value string) string {
	if isPendingValue(value) {
		return "Em correção"
	}
	if isGradeLabel(label) {
		score, ok := parseScore(value)
		if !ok {
			return "Em correção"
		}
		return formatGradeNumber(score)
	}
	return value
}

func isPendingValue(value string) bool {
	text := normalizeHeader(value)
	return text == "" ||
		strings.Contains(text, "nao corrigid") ||
		strings.Contains(text, "em correcao") ||
		strings.Contains(text, "nao foi lancad")
}

func scoreTone(label string, value string) string {
	score, ok := parseScore(value)
	if !ok {
		if strings.TrimSpace(value) == "" && isGradeLabel(label) {
			return "score-pending"
		}
		return ""
	}
	if !isGradeLabel(label) {
		return ""
	}
	if score <= 1 {
		return scoreToneFromRatio(score*100, false)
	}
	return scoreToneFromRatio((score/10)*100, false)
}

func scoreToneFromRatio(ratio float64, pending bool) string {
	if pending {
		return "score-pending"
	}
	if ratio <= 30 {
		return "score-danger"
	}
	if ratio < 70 {
		return "score-warning"
	}
	return "score-success"
}

func activityCardTone(status string, tone string) string {
	if normalizeHeader(status) != "encerrado" {
		return "score-pending"
	}
	return tone
}

func summaryCardLabel(header string) string {
	label := normalizeHeader(header)
	switch {
	case strings.Contains(label, "prova"):
		return "Prova AB"
	case isAverageColumn(header):
		return "Média AB"
	case isActivityColumn(header):
		return activityLabel(header)
	case label == "total":
		return "Total"
	case strings.Contains(label, "projeto"):
		return "Projeto"
	case strings.Contains(label, "trabalho"):
		return "Trabalho"
	default:
		return humanizeLabel(header)
	}
}

func cardLabel(header string) string {
	label := normalizeHeader(header)
	switch {
	case strings.HasPrefix(label, "semana"):
		return humanizeLabel(header)
	case isQuestionLabel(label):
		return questionLabel(label)
	case label == "sgbd":
		return "SGBD"
	case label == "dataset":
		return "Dataset"
	case label == "crud":
		return "CRUD"
	case strings.Contains(label, "apresentacao"):
		return "Apresentação"
	case strings.Contains(label, "organizacao"):
		return "Organização"
	case strings.Contains(label, "referencias"):
		return "Referências"
	case strings.Contains(label, "discussao"):
		return "Discussão em aula"
	default:
		return humanizeLabel(header)
	}
}

func humanizeLabel(label string) string {
	words := strings.Fields(strings.TrimSpace(label))
	for idx, word := range words {
		normalized := normalizeHeader(word)
		switch {
		case normalized == "at" || normalized == "at.":
			words[idx] = "AT."
		case isQuestionLabel(normalized):
			words[idx] = questionLabel(normalized)
		case normalized == "sgbd":
			words[idx] = "SGBD"
		case normalized == "crud":
			words[idx] = "CRUD"
		case normalized == "dataset":
			words[idx] = "Dataset"
		case normalized == "apresentacao":
			words[idx] = "Apresentação"
		case normalized == "organizacao":
			words[idx] = "Organização"
		case normalized == "referencias":
			words[idx] = "Referências"
		case normalized == "discussao":
			words[idx] = "Discussão"
		case normalized == "nota":
			words[idx] = "Nota"
		case normalized == "media":
			words[idx] = "Média"
		case normalized == "total":
			words[idx] = "Total"
		case normalized == "semana":
			words[idx] = "Semana"
		}
	}
	return strings.Join(words, " ")
}

func activityLabel(label string) string {
	normalized := normalizeHeader(label)
	switch {
	case strings.HasPrefix(normalized, "at."):
		return "Atividade " + strings.TrimSpace(strings.TrimPrefix(normalized, "at."))
	case strings.HasPrefix(normalized, "at "):
		return "Atividade " + strings.TrimSpace(strings.TrimPrefix(normalized, "at "))
	case strings.Contains(normalized, "atividade"):
		suffix := strings.TrimSpace(strings.TrimPrefix(normalized, "atividade"))
		if suffix != "" {
			return "Atividade " + suffix
		}
	}
	return strings.ToUpper(strings.TrimSpace(label))
}

func questionLabel(label string) string {
	label = normalizeHeader(label)
	switch {
	case strings.HasPrefix(label, "questao "):
		return "Questão " + strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(label, "questao ")))
	case strings.HasPrefix(label, "q."):
		return strings.ToUpper(label)
	case strings.HasPrefix(label, "q"):
		return "Q. " + strings.TrimSpace(strings.TrimPrefix(label, "q"))
	default:
		return strings.ToUpper(label)
	}
}

func isQuestionLabel(label string) bool {
	label = normalizeHeader(label)
	if strings.HasPrefix(label, "questao ") {
		return true
	}
	if strings.HasPrefix(label, "q.") {
		return true
	}
	if strings.HasPrefix(label, "q ") {
		return true
	}
	runes := []rune(label)
	return len(runes) > 1 && runes[0] == 'q' && unicode.IsDigit(runes[1])
}

func shouldShowColumn(header string) bool {
	label := normalizeHeader(header)
	return label != "" &&
		label != "grupo" &&
		label != "equipe" &&
		!strings.Contains(label, "matricula") &&
		!strings.Contains(label, "nome do aluno") &&
		label != "nome" &&
		label != "aluno"
}

func shouldShowMainCard(header string) bool {
	if !shouldShowColumn(header) || isDetailOnlyColumn(header) {
		return false
	}
	label := normalizeHeader(header)
	if strings.Contains(label, "at. 4") || strings.Contains(label, "atividade 4") {
		return true
	}
	return label == "nota" ||
		strings.Contains(label, "prova") ||
		label == "total" ||
		strings.Contains(label, "media") ||
		isActivityColumn(header) ||
		strings.Contains(label, "projeto") ||
		label == "ab1" ||
		label == "ab2"
}

func projectMainColumn(header string) bool {
	if !shouldShowColumn(header) {
		return false
	}
	label := normalizeHeader(header)
	return label == "nota" ||
		label == "total" ||
		strings.Contains(label, "media") ||
		label == "projeto" ||
		label == "trabalho" ||
		strings.HasPrefix(label, "projeto ab") ||
		strings.HasPrefix(label, "nota projeto") ||
		strings.HasPrefix(label, "trabalho ab") ||
		strings.HasPrefix(label, "nota trabalho")
}

func projectDetailColumn(header string) bool {
	return shouldShowColumn(header) && !projectMainColumn(header)
}

func isDetailOnlyColumn(header string) bool {
	label := normalizeHeader(header)
	return strings.HasPrefix(label, "semana") ||
		label == "sgbd" ||
		label == "dataset" ||
		label == "crud" ||
		strings.Contains(label, "apresentacao") ||
		strings.Contains(label, "organizacao") ||
		isQuestionLabel(label)
}

func isActivityColumn(header string) bool {
	label := normalizeHeader(header)
	return strings.HasPrefix(label, "at.") || strings.HasPrefix(label, "at ") || strings.Contains(label, "atividade")
}

func isAverageColumn(header string) bool {
	label := normalizeHeader(header)
	return strings.Contains(label, "media")
}

func isProofColumn(header string) bool {
	label := normalizeHeader(header)
	return strings.Contains(label, "prova")
}

func isGradeLabel(label string) bool {
	normalized := normalizeHeader(label)
	return normalized == "nota" ||
		strings.Contains(normalized, "prova") ||
		strings.Contains(normalized, "nota ab") ||
		strings.Contains(normalized, "somatorio") ||
		normalized == "total" ||
		strings.Contains(normalized, "media") ||
		strings.Contains(normalized, "projeto") ||
		strings.Contains(normalized, "trabalho") ||
		isActivityColumn(label)
}

func parseScore(value string) (float64, bool) {
	text := strings.ReplaceAll(strings.TrimSpace(value), ",", ".")
	var builder strings.Builder
	for _, char := range text {
		if unicode.IsDigit(char) || char == '.' || char == '-' {
			builder.WriteRune(char)
		}
	}
	clean := builder.String()
	if clean == "" || clean == "-" || clean == "." {
		return 0, false
	}
	return parseNumber(clean)
}

func formatScore(value float64) string {
	return formatNumber(value)
}

func formatGradeNumber(value float64) string {
	return formatNumberFixed(value, 2)
}

func scoreComparisonDisplay(obtained float64, maximum float64) string {
	return formatGradeNumber(obtained) + " de " + formatGradeNumber(maximum)
}

func canonicalCriterionMaximum(label string, sourceMaximum float64) float64 {
	if maximum := inferMaxForLabel(label); maximum > 0 {
		return maximum
	}
	return sourceMaximum
}

func usesOfficialQuestionWeights(labels []string) bool {
	seen := [6]bool{}
	questions := 0
	for _, label := range labels {
		switch questionScoreKey(label) {
		case "q.1":
			seen[0] = true
		case "q.2":
			seen[1] = true
		case "q.3":
			seen[2] = true
		case "q.4":
			seen[3] = true
		case "q.5":
			seen[4] = true
		case "q.6":
			seen[5] = true
		}
	}
	for _, found := range seen {
		if found {
			questions++
		}
	}
	return questions >= 4
}

func inferMaxForLabel(label string) float64 {
	normalized := normalizeHeader(label)
	if key := questionScoreKey(normalized); key != "" {
		if value, ok := inferredCriterionMaxima[key]; ok {
			return value
		}
	}
	for key, value := range inferredCriterionMaxima {
		if strings.Contains(normalized, key) {
			return value
		}
	}
	return 0
}

func compareDetailLabels(left string, right string) int {
	order := []string{"organizacao", "q.1", "q.2", "q.3", "q.4", "q.5", "q.6", "semana 1", "semana 2", "semana 3", "semana 4", "sgbd", "dataset", "crud", "apresentacao", "referencias", "discussao"}
	leftLabel := normalizeHeader(left)
	rightLabel := normalizeHeader(right)
	if key := questionScoreKey(leftLabel); key != "" {
		leftLabel = key
	}
	if key := questionScoreKey(rightLabel); key != "" {
		rightLabel = key
	}
	leftIdx := orderIndex(order, leftLabel)
	rightIdx := orderIndex(order, rightLabel)
	if leftIdx != rightIdx {
		if leftIdx == -1 {
			return 1
		}
		if rightIdx == -1 {
			return -1
		}
		return leftIdx - rightIdx
	}
	return strings.Compare(leftLabel, rightLabel)
}

func questionScoreKey(label string) string {
	label = normalizeHeader(label)
	label = strings.TrimSpace(strings.TrimPrefix(label, "questao"))
	label = strings.TrimSpace(strings.TrimPrefix(label, "q."))
	label = strings.TrimSpace(strings.TrimPrefix(label, "q"))
	label = strings.Trim(label, ". ")
	if label == "" {
		return ""
	}
	if len([]rune(label)) == 1 {
		switch label {
		case "a":
			return "q.1"
		case "b":
			return "q.2"
		case "c":
			return "q.3"
		case "d":
			return "q.4"
		case "e":
			return "q.5"
		case "f":
			return "q.6"
		}
	}
	if _, ok := parseNumber(label); ok {
		return "q." + label
	}
	return ""
}

func orderIndex(order []string, label string) int {
	for idx, item := range order {
		if strings.Contains(label, item) {
			return idx
		}
	}
	return -1
}
