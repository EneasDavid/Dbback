package app

type GradeResult struct {
	Exam          string         `json:"exam"`
	Matricula     string         `json:"matricula"`
	Name          string         `json:"name"`
	Tables        []TableResult  `json:"tables"`
	StudentStatus *StudentStatus `json:"studentStatus,omitempty"`
}

type TableResult struct {
	Key       string       `json:"key"`
	Label     string       `json:"label"`
	SheetName string       `json:"sheetName"`
	Kind      string       `json:"kind"`
	Complete  bool         `json:"complete"`
	Status    string       `json:"status,omitempty"`
	Cards     []CardResult `json:"cards"`
}

type CardResult struct {
	Key           string         `json:"key"`
	Label         string         `json:"label"`
	Value         string         `json:"value"`
	DisplayValue  string         `json:"displayValue"`
	Tone          string         `json:"tone,omitempty"`
	Comment       string         `json:"comment,omitempty"`
	CommentAuthor string         `json:"commentAuthor,omitempty"`
	Details       []DetailResult `json:"details,omitempty"`
}

type DetailResult struct {
	Key           string  `json:"key"`
	Label         string  `json:"label"`
	Value         string  `json:"value"`
	Max           float64 `json:"max"`
	DisplayScore  string  `json:"displayScore"`
	Ratio         float64 `json:"ratio"`
	Pending       bool    `json:"pending"`
	Tone          string  `json:"tone,omitempty"`
	Comment       string  `json:"comment,omitempty"`
	CommentAuthor string  `json:"commentAuthor,omitempty"`
}

type StudentStatus struct {
	AB1      float64 `json:"ab1"`
	AB2      float64 `json:"ab2"`
	Average  float64 `json:"average"`
	Approved bool    `json:"approved"`
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

type SheetComment struct {
	Cell   string
	Text   string
	Author string
}
