package app

import "testing"

func TestV2ABStateUsesActiveColumn(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"AB", "Nome", "Ativa"},
		rows: [][]string{
			{"AB1", "AB1", "sim"},
			{"AB2", "AB2", "não"},
		},
	}

	label, active := v2ABState(grid, "ab2")

	if label != "AB2" || active {
		t.Fatalf("v2ABState() = %q/%v, want AB2/false", label, active)
	}
}

func TestV2ActivitiesForABUsesABAndWeight(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Atividade", "AB", "Peso Máximo", "Aba", "Ativa"},
		rows: [][]string{
			{"Modelo", "AB2", "3", "Projeto", "sim"},
			{"Pesquisa", "AB1", "2", "", "sim"},
			{"Lista", "AB1", "1,5", "AT. Lista", "não"},
		},
		schemaStatus:  "v2",
		spreadsheetID: "sheet-a",
	}

	activities := v2ActivitiesForAB(grid, "ab1")

	if len(activities) != 1 {
		t.Fatalf("activities len = %d, want 1: %#v", len(activities), activities)
	}
	if activities[0].Label != "Pesquisa" || activities[0].SheetName != "Pesquisa" || activities[0].Weight != 2 {
		t.Fatalf("unexpected activity: %#v", activities[0])
	}
}

func TestV2ActivityItemsNormalizesCriteriaByWeightAndKeepsComments(t *testing.T) {
	grid := &sheetGrid{
		headers: []string{"Matrícula", "Critério A", "Critério B"},
		rows: [][]string{
			{"Nota máxima", "2", "3"},
			{"123", "1", "3"},
		},
		rowNotes: [][]string{
			{"", "", ""},
			{"", "comentário A", "comentário B"},
		},
		rowNoteAuthors: [][]string{
			{"", "", ""},
			{"", "Prof", "Prof"},
		},
	}

	items := v2ActivityItems(grid, 0, 1, 2)

	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2: %#v", len(items), items)
	}
	if items[0].NotaMaxima != "0,8" || items[0].NotaAlcancada != "0,4" {
		t.Fatalf("first normalized item = %#v, want max 0,8 and value 0,4", items[0])
	}
	if items[1].Comment != "comentário B" || items[1].CommentAuthor != "Prof" {
		t.Fatalf("second item comment = %#v", items[1])
	}
}

func TestV2ActivityLaunchedSkipsNotLaunchedText(t *testing.T) {
	activity := v2ActivityConfig{SummaryCol: 1}
	row := []string{"123", "Essa atividade não foi lançada"}

	if v2ActivityLaunched(row, activity) {
		t.Fatal("v2ActivityLaunched() = true, want false")
	}
}
