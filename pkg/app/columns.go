package app

import "strings"

func includeColumn(kind string, header string) bool {
	if strings.TrimSpace(header) == "" {
		return false
	}
	switch kind {
	case "summary":
		return true
	case "ab2summary":
		normalized := normalizeHeader(header)
		return normalized == "atividade" ||
			normalized == "projeto" ||
			normalized == "total" ||
			strings.Contains(normalized, "nota") ||
			strings.Contains(normalized, "media") ||
			isActivityColumn(header)
	default:
		return true
	}
}

func tableComplete(grid *sheetGrid, table TableConfig) bool {
	var idx int
	switch table.Kind {
	case "summary", "ab2summary":
		idx = totalABColumn(grid.headers)
	case "project":
		idx = totalColumn(grid.headers)
	default:
		return true
	}
	if idx < 0 {
		return false
	}
	nameIdx := nameColumn(grid.headers)
	matriculaIdx := matriculaColumn(grid.headers)
	for _, row := range grid.rows {
		if !studentRow(row, nameIdx, matriculaIdx) {
			continue
		}
		if strings.TrimSpace(valueAt(row, idx)) == "" {
			return false
		}
	}
	return true
}

func matriculaColumn(headers []string) int {
	candidates := []string{"matricula", "matrícula", "mat", "registro", "ra"}
	for idx, header := range headers {
		normalized := normalizeHeader(header)
		for _, candidate := range candidates {
			if normalized == normalizeHeader(candidate) {
				return idx
			}
		}
	}
	for idx, header := range headers {
		if strings.Contains(normalizeHeader(header), "matric") {
			return idx
		}
	}
	return -1
}

func nameColumn(headers []string) int {
	candidates := []string{"nome", "aluno", "estudante", "discente", "nome completo", "nome do aluno", "nome do aluno(a)"}
	for idx, header := range headers {
		normalized := normalizeHeader(header)
		for _, candidate := range candidates {
			if normalized == normalizeHeader(candidate) {
				return idx
			}
		}
	}
	for idx, header := range headers {
		normalized := normalizeHeader(header)
		if strings.Contains(normalized, "nome") || strings.Contains(normalized, "aluno") {
			return idx
		}
	}
	return -1
}

func groupColumn(headers []string) int {
	for idx, header := range headers {
		normalized := normalizeHeader(header)
		if normalized == "grupo" || normalized == "equipe" || strings.Contains(normalized, "grupo") || strings.Contains(normalized, "equipe") {
			return idx
		}
	}
	return -1
}

func exactNameColumn(headers []string) int {
	candidates := []string{"nome", "aluno", "estudante", "discente", "nome completo", "nome do aluno", "nome do aluno(a)"}
	for idx, header := range headers {
		normalized := normalizeHeader(header)
		for _, candidate := range candidates {
			if normalized == normalizeHeader(candidate) {
				return idx
			}
		}
	}
	return -1
}

func headerScore(headers []string) int {
	score := 0
	if matriculaColumn(headers) >= 0 {
		score += 3
	}
	if exactNameColumn(headers) >= 0 {
		score += 4
	} else if nameColumn(headers) >= 0 {
		score++
	}
	for _, header := range headers {
		if summaryColumn(header) {
			score += 2
		}
	}
	return score
}

func summaryColumn(header string) bool {
	normalized := normalizeHeader(header)
	if strings.Contains(normalized, "nota") && strings.Contains(normalized, "ab") {
		return true
	}
	return strings.Contains(normalized, "prova") && strings.Contains(normalized, "ab")
}

func totalABColumn(headers []string) int {
	for idx, header := range headers {
		normalized := normalizeHeader(header)
		if strings.Contains(normalized, "nota") && strings.Contains(normalized, "ab") {
			return idx
		}
	}
	return -1
}

func totalColumn(headers []string) int {
	for idx, header := range headers {
		if normalizeHeader(header) == "total" {
			return idx
		}
	}
	return -1
}
