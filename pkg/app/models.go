package app

type ColumnResult struct {
	Key           string `json:"key"`
	Label         string `json:"label"`
	Value         string `json:"value"`
	Comment       string `json:"comment,omitempty"`
	CommentAuthor string `json:"commentAuthor,omitempty"`
}

type ActivityItem struct {
	Key             string `json:"key"`
	Subtopic        string `json:"subtopic"`
	NotaMaxima      string `json:"notaMaxima"`
	NotaAlcancada   string `json:"notaAlcancada"`
	Comentario      string `json:"comentario,omitempty"`
	ComentarioAutor string `json:"comentarioAutor,omitempty"`
}

type GradeResult struct {
	Exam      string        `json:"exam"`
	Matricula string        `json:"matricula"`
	Name      string        `json:"name"`
	Tables    []TableResult `json:"tables"`
}

type TableResult struct {
	Key       string         `json:"key"`
	Label     string         `json:"label"`
	SheetName string         `json:"sheetName"`
	Kind      string         `json:"kind"`
	Complete  bool           `json:"complete"`
	Columns   []ColumnResult `json:"columns"`
	Items     []ActivityItem `json:"items,omitempty"`
}

type LoginIdentity struct {
	Matricula string `json:"matricula"`
	Name      string `json:"name"`
}

type sheetGrid struct {
	headers        []string
	notes          []string
	noteAuthors    []string
	rows           [][]string
	rowNotes       [][]string
	rowNoteAuthors [][]string
	headerRow      int
	rowIndices     []int
}

type cellComment struct {
	Text   string
	Author string
}
