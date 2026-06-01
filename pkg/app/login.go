package app

import (
	"context"
	"strings"
)

func (c *SheetsClient) LoginIdentity(ctx context.Context, matricula string) (LoginIdentity, error) {
	grid, err := c.loadSheet(ctx, c.cfg.LoginSheet)
	if err != nil {
		return LoginIdentity{}, NewHTTPError(503, "não conseguiu acessar a planilha de login; verifique GOOGLE_SHEET_ID/GOOGLE_SHEET_IDS, credencial da service account e compartilhamento da planilha")
	}
	matriculaIdx := matriculaColumn(grid.headers)
	nameIdx := nameColumn(grid.headers)
	if matriculaIdx < 0 || nameIdx < 0 {
		return LoginIdentity{}, NewHTTPError(503, "a aba de login precisa ter colunas de matricula e nome")
	}
	for rowIdx, row := range grid.rows {
		if matriculaIdx < len(row) && sameLookupValue(row[matriculaIdx], matricula, false) {
			name := valueAt(row, nameIdx)
			if strings.TrimSpace(name) == "" {
				return LoginIdentity{}, NewHTTPError(401, "não achou o usuário")
			}
			return LoginIdentity{
				Matricula:     valueAt(row, matriculaIdx),
				Name:          name,
				SpreadsheetID: grid.rowSource(rowIdx),
				SchemaStatus:  grid.rowSchema(rowIdx),
			}, nil
		}
	}
	return LoginIdentity{}, NewHTTPError(401, "não achou o usuário")
}
