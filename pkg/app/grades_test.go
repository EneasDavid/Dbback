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
