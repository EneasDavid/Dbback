package app

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

type activityItem struct {
	Key             string
	Subtopic        string
	NotaMaxima      string
	NotaAlcancada   string
	Comentario      string
	ComentarioAutor string
}

type studentCell struct {
	ColIdx        int
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
		obtained, hasObtained := parseScore(item.NotaAlcancada)
		pending := strings.TrimSpace(item.NotaAlcancada) == ""
		ratio := 0.0
		if !pending && hasObtained && maximum > 0 {
			ratio = math.Min((obtained/maximum)*100, 100)
		}
		details = append(details, DetailResult{
			Key:           item.Key,
			Label:         humanizeLabel(item.Subtopic),
			Value:         item.NotaAlcancada,
			Max:           maximum,
			DisplayScore:  detailDisplayScore(item.NotaAlcancada, maximum, pending),
			Ratio:         ratio,
			Pending:       pending,
			Tone:          scoreToneFromRatio(ratio, pending),
			Comment:       item.Comentario,
			CommentAuthor: item.ComentarioAutor,
		})
	}
	return details
}

func columnDetails(cells []studentCell) []DetailResult {
	details := make([]DetailResult, 0, len(cells))
	for _, cell := range cells {
		if !isDetailOnlyColumn(cell.Header) {
			continue
		}
		maximum := inferMaxForLabel(cell.Header)
		if maximum <= 0 {
			maximum = 1
		}
		obtained, hasObtained := parseScore(cell.Value)
		pending := strings.TrimSpace(cell.Value) == ""
		ratio := 0.0
		if !pending && hasObtained && maximum > 0 {
			ratio = math.Min((obtained/maximum)*100, 100)
		}
		details = append(details, DetailResult{
			Key:           cell.Key,
			Label:         cell.Label,
			Value:         cell.Value,
			Max:           maximum,
			DisplayScore:  detailDisplayScore(cell.Value, maximum, pending),
			Ratio:         ratio,
			Pending:       pending,
			Tone:          scoreToneFromRatio(ratio, pending),
			Comment:       cell.Comment,
			CommentAuthor: cell.CommentAuthor,
		})
	}
	sort.SliceStable(details, func(i, j int) bool {
		return compareDetailLabels(details[i].Label, details[j].Label) < 0
	})
	return details
}

func detailDisplayScore(value string, maximum float64, pending bool) string {
	if pending {
		return "Não corrigido ainda"
	}
	if obtained, ok := parseScore(value); ok && maximum > 0 {
		return formatScore(obtained) + " / " + formatScore(maximum)
	}
	if maximum > 0 {
		return "Max " + formatScore(maximum)
	}
	return strings.TrimSpace(value)
}

func displayValue(label string, value string) string {
	if isPendingValue(value) {
		return "Não corrigida ainda"
	}
	if isGradeLabel(label) {
		if _, ok := parseScore(value); !ok {
			return "Não corrigida ainda"
		}
	}
	return value
}

func isPendingValue(value string) bool {
	text := normalizeHeader(value)
	return text == "" || strings.Contains(text, "nao corrigid")
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
		score *= 10
	}
	if score < 5 {
		return "score-danger"
	}
	if score < 7 {
		return "score-warning"
	}
	return "score-success"
}

func scoreToneFromRatio(ratio float64, pending bool) string {
	if pending {
		return "score-pending"
	}
	if ratio < 50 {
		return "score-danger"
	}
	if ratio < 70 {
		return "score-warning"
	}
	return "score-success"
}

func summaryCardLabel(header string) string {
	label := normalizeHeader(header)
	switch {
	case strings.Contains(label, "prova"):
		return "Nota da prova"
	case isAverageColumn(header):
		return "Média da AB"
	case isActivityColumn(header):
		return activityLabel(header)
	case label == "total":
		return "Total"
	case strings.Contains(label, "projeto"):
		return "Projeto"
	default:
		return humanizeLabel(header)
	}
}

func cardLabel(header string) string {
	label := normalizeHeader(header)
	switch {
	case strings.HasPrefix(label, "semana"):
		return humanizeLabel(header)
	case strings.HasPrefix(label, "q."):
		return questionLabel(label)
	case strings.HasPrefix(label, "q"):
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
		case strings.HasPrefix(normalized, "q"):
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
		return "AT. " + strings.TrimSpace(strings.TrimPrefix(normalized, "at."))
	case strings.HasPrefix(normalized, "at "):
		return "AT. " + strings.TrimSpace(strings.TrimPrefix(normalized, "at "))
	case strings.Contains(normalized, "atividade"):
		suffix := strings.TrimSpace(strings.TrimPrefix(normalized, "atividade"))
		if suffix != "" {
			return "AT. " + suffix
		}
	}
	return strings.ToUpper(strings.TrimSpace(label))
}

func questionLabel(label string) string {
	switch {
	case strings.HasPrefix(label, "q."):
		return strings.ToUpper(label)
	case strings.HasPrefix(label, "q"):
		return "Q. " + strings.TrimSpace(strings.TrimPrefix(label, "q"))
	default:
		return strings.ToUpper(label)
	}
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

func isDetailOnlyColumn(header string) bool {
	label := normalizeHeader(header)
	return strings.HasPrefix(label, "semana") ||
		label == "sgbd" ||
		label == "dataset" ||
		label == "crud" ||
		strings.Contains(label, "apresentacao") ||
		strings.Contains(label, "organizacao") ||
		strings.Contains(label, "q.") ||
		strings.HasPrefix(label, "q")
}

func isActivityColumn(header string) bool {
	label := normalizeHeader(header)
	return strings.HasPrefix(label, "at.") || strings.HasPrefix(label, "at ") || strings.Contains(label, "atividade")
}

func isAverageColumn(header string) bool {
	label := normalizeHeader(header)
	return strings.Contains(label, "media")
}

func isGradeLabel(label string) bool {
	normalized := normalizeHeader(label)
	return normalized == "nota" ||
		strings.Contains(normalized, "prova") ||
		strings.Contains(normalized, "nota ab") ||
		normalized == "total" ||
		strings.Contains(normalized, "media") ||
		strings.Contains(normalized, "projeto") ||
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

func inferMaxForLabel(label string) float64 {
	normalized := normalizeHeader(label)
	values := map[string]float64{
		"organizacao":  0.5,
		"q.1":          1.5,
		"q.2":          1,
		"q.3":          1.5,
		"q.4":          2,
		"q.5":          1.5,
		"q.6":          2,
		"semana 1":     0.25,
		"semana 2":     0.25,
		"semana 3":     0.25,
		"semana 4":     0.25,
		"sgbd":         1,
		"dataset":      1,
		"crud":         1,
		"apresentacao": 2,
	}
	for key, value := range values {
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

func orderIndex(order []string, label string) int {
	for idx, item := range order {
		if strings.Contains(label, item) {
			return idx
		}
	}
	return -1
}
