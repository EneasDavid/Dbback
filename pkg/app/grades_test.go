package app

import "testing"

// TestAddAB1ScoreAverageAddsVisibleScoreCards testa se a média da AB1 é calculada
// somando as notas das atividades com a nota da prova corretamente.
func TestAddAB1ScoreAverageAddsVisibleScoreCards(t *testing.T) {
	result := GradeResult{
		Exam: "AB1",
		Tables: []TableResult{
			{Kind: "activity", Cards: []CardResult{{Label: "Nota", Value: "0,98"}}},
			{Kind: "activity", Cards: []CardResult{{Label: "Nota", Value: "0,85"}}},
			{Kind: "activity", Cards: []CardResult{{Label: "Nota", Value: "0,65"}}},
			{Kind: "summary", Cards: []CardResult{
				makeCard("prova", "Prova AB", "6", "", "", nil),
			}},
		},
	}

	addAB1ScoreAverage(&result)

	// Esperamos 5 tabelas: 3 atividades + 1 summary (prova) + 1 média final da AB1
	if len(result.Tables) != 5 {
		t.Fatalf("tables len = %d, want 5: %#v", len(result.Tables), result.Tables)
	}
	
	summary := result.Tables[4]
	if summary.Key != "media-ab1" || summary.Kind != "ab1summary" || summary.Label != "Média AB1" {
		t.Fatalf("unexpected AB1 summary table: %#v", summary)
	}
	
	// A soma deve ser: 0.98 + 0.85 + 0.65 + 6 = 8.48
	if len(summary.Cards) != 1 || summary.Cards[0].Label != "" || summary.Cards[0].Value != "8,48" {
		t.Fatalf("unexpected AB1 summary card: %#v", summary.Cards)
	}
}

// TestAddAB1ScoreAverageCapsAtTen garante que a Média da AB1 não ultrapasse 10.
func TestAddAB1ScoreAverageCapsAtTen(t *testing.T) {
	result := GradeResult{
		Exam: "AB1",
		Tables: []TableResult{
			{Kind: "activity", Cards: []CardResult{{Label: "Nota", Value: "0,98"}}},
			{Kind: "activity", Cards: []CardResult{{Label: "Nota", Value: "0,85"}}},
			{Kind: "activity", Cards: []CardResult{{Label: "Nota", Value: "0,65"}}},
			{Kind: "summary", Cards: []CardResult{
				makeCard("prova", "Prova AB", "8", "", "", nil),
			}},
		},
	}

	addAB1ScoreAverage(&result)

	if got := result.Tables[4].Cards[0].Value; got != "10" {
		t.Fatalf("capped AB1 value = %q, want 10", got)
	}
}

// TestAddAB1ScoreAverageAppendsMediaAsLastTable garante que a tabela de média
// seja sempre anexada como o último elemento do slice de tabelas.
func TestAddAB1ScoreAverageAppendsMediaAsLastTable(t *testing.T) {
	result := GradeResult{
		Exam: "AB1",
		Tables: []TableResult{
			{Kind: "activity", Cards: []CardResult{{Label: "Nota", Value: "0,98"}}},
			{Kind: "summary", Cards: []CardResult{
				makeCard("prova", "Prova AB", "5", "", "", nil),
			}},
			{Kind: "activity", Key: "extra", Cards: []CardResult{{Label: "Extra", Value: "0,5"}}},
		},
	}

	addAB1ScoreAverage(&result)

	last := result.Tables[len(result.Tables)-1]
	if last.Kind != "ab1summary" || last.Key != "media-ab1" {
		t.Fatalf("last table = %#v, want media-ab1", last)
	}
}

// TestAddAB2ScoreAverageAddsVisibleScoreCards valida o cálculo de média para AB2.
func TestAddAB2ScoreAverageAddsVisibleScoreCards(t *testing.T) {
	result := GradeResult{
		Exam: "AB2",
		Tables: []TableResult{
			{Key: "at4", Kind: "activity", Cards: []CardResult{
				makeCard("nota", "Nota", "0,65", "", "", nil),
			}},
			{Key: "projeto", Kind: "project", Cards: []CardResult{
				makeCard("total", "Total", "0,45", "", "", nil),
			}},
		},
	}

	addAB2ScoreAverage(&result)

	if len(result.Tables) != 3 {
		t.Fatalf("tables len = %d, want 3: %#v", len(result.Tables), result.Tables)
	}
	summary := result.Tables[2]
	if summary.Key != "media-ab2" || summary.Kind != "ab2summary" || summary.Label != "Média AB2" {
		t.Fatalf("unexpected AB2 summary table: %#v", summary)
	}
	if len(summary.Cards) != 1 || summary.Cards[0].Label != "" || summary.Cards[0].Value != "1,1" {
		t.Fatalf("unexpected AB2 summary card: %#v", summary.Cards)
	}
}

// TestAddAB2ScoreAverageCapsAtTen garante que a Média da AB2 seja limitada a 10.
func TestAddAB2ScoreAverageCapsAtTen(t *testing.T) {
	result := GradeResult{
		Exam: "AB2",
		Tables: []TableResult{
			{Key: "at4", Kind: "activity", Cards: []CardResult{{Label: "Nota", Value: "8"}}},
			{Key: "projeto", Kind: "project", Cards: []CardResult{{Label: "Total", Value: "4"}}},
		},
	}

	addAB2ScoreAverage(&result)

	if got := result.Tables[2].Cards[0].Value; got != "10" {
		t.Fatalf("capped AB2 value = %q, want 10", got)
	}
}