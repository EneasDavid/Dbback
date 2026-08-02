package app

import "strings"

// matriculaColumn e nameColumn usam apenas correspondencia exata contra os
// nomes aceitos - cabecalhos de login/identificacao do aluno sao obrigatorios
// e nao devem ser adivinhados por aproximacao de texto.
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

// headerScore ajuda parseGrid a localizar a linha de cabecalho real dentro
// de uma aba (linhas de titulo/instrucoes acima dela pontuam menos) - isso e
// deteccao de posicao da linha, nao adivinhacao de qual coluna e qual, entao
// fica fora do escopo de "cabecalho obrigatorio".
func headerScore(headers []string) int {
	score := 0
	if matriculaColumn(headers) >= 0 {
		score += 3
	}
	if nameColumn(headers) >= 0 {
		score += 4
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
