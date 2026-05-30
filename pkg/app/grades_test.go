package app

import "testing"

func TestAddAB1ScoreSumAddsActivitiesAndProof(t *testing.T) {
	result := GradeResult{
		Exam: "AB1",
		Tables: []TableResult{
			{Kind: "activity", Cards: []CardResult{{Value: "0,98"}}},
			{Kind: "activity", Cards: []CardResult{{Value: "0,85"}}},
			{Kind: "activity", Cards: []CardResult{{Value: "0,65"}}},
			{Kind: "summary", Cards: []CardResult{
				makeCard("prova", "Prova AB", "6", "", "", nil),
			}},
		},
	}

	addAB1ScoreSum(&result)

	cards := result.Tables[3].Cards
	if len(cards) != 2 {
		t.Fatalf("summary cards len = %d, want 2: %#v", len(cards), cards)
	}
	if cards[0].Label != "Prova AB" || cards[1].Label != "Somatório AB" {
		t.Fatalf("unexpected summary card order: %#v", cards)
	}
	if got := cards[1].Value; got != "8,48" {
		t.Fatalf("sum value = %q, want 8,48", got)
	}
}

func TestAddAB1ScoreSumCapsAtTen(t *testing.T) {
	result := GradeResult{
		Exam: "AB1",
		Tables: []TableResult{
			{Kind: "activity", Cards: []CardResult{{Value: "0,98"}}},
			{Kind: "activity", Cards: []CardResult{{Value: "0,85"}}},
			{Kind: "activity", Cards: []CardResult{{Value: "0,65"}}},
			{Kind: "summary", Cards: []CardResult{
				makeCard("prova", "Prova AB", "8", "", "", nil),
			}},
		},
	}

	addAB1ScoreSum(&result)

	if got := result.Tables[3].Cards[1].Value; got != "10" {
		t.Fatalf("capped sum value = %q, want 10", got)
	}
}

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

	addAB1ScoreSum(&result)
	addAB1ScoreAverage(&result)

	if len(result.Tables) != 5 {
		t.Fatalf("tables len = %d, want 5: %#v", len(result.Tables), result.Tables)
	}
	summary := result.Tables[4]
	if summary.Key != "media-ab1" || summary.Kind != "ab1summary" || summary.Label != "Média AB1" {
		t.Fatalf("unexpected AB1 summary table: %#v", summary)
	}
	if len(summary.Cards) != 1 || summary.Cards[0].Label != "" || summary.Cards[0].Value != "8,48" {
		t.Fatalf("unexpected AB1 summary card: %#v", summary.Cards)
	}
}

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

	addAB1ScoreSum(&result)
	addAB1ScoreAverage(&result)

	if got := result.Tables[4].Cards[0].Value; got != "10" {
		t.Fatalf("capped AB1 value = %q, want 10", got)
	}
}

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
