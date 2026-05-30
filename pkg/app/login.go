package app

import (
	"context"
	"strings"
)

func (c *SheetsClient) LoginIdentity(ctx context.Context, matricula string) (LoginIdentity, error) {
	grid, err := c.loadSheet(ctx, c.cfg.LoginSheet)
	if err != nil {
		return LoginIdentity{}, NewHTTPError(503, "não conseguiu logar na planilha")
	}
	matriculaIdx := matriculaColumn(grid.headers)
	nameIdx := nameColumn(grid.headers)
	if matriculaIdx < 0 || nameIdx < 0 {
		return LoginIdentity{}, NewHTTPError(503, "não conseguiu logar na planilha")
	}
	for _, row := range grid.rows {
		if matriculaIdx < len(row) && normalizeID(row[matriculaIdx]) == normalizeID(matricula) {
			name := valueAt(row, nameIdx)
			if strings.TrimSpace(name) == "" {
				return LoginIdentity{}, NewHTTPError(401, "não achou o usuário")
			}
			return LoginIdentity{Matricula: valueAt(row, matriculaIdx), Name: name}, nil
		}
	}
	return LoginIdentity{}, NewHTTPError(401, "não achou o usuário")
}
