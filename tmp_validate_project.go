package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"feedback/pkg/app"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	cfg := app.LoadConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client, err := app.NewSheetsClient(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	identity, err := client.LoginIdentity(ctx, "2025026109")
	if err != nil {
		log.Fatal(err)
	}
	result, err := client.GradeFor(ctx, "ab2", app.SessionUser{
		Matricula:     identity.Matricula,
		Name:          identity.Name,
		SpreadsheetID: identity.SpreadsheetID,
		SchemaStatus:  identity.SchemaStatus,
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, table := range result.Tables {
		if table.SheetName != "Projeto" && table.Label != "Projeto" && table.Label != "projeto" {
			continue
		}
		data, _ := json.MarshalIndent(table, "", "  ")
		fmt.Println(string(data))
	}
}
