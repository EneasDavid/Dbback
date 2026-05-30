package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"feedback/pkg/app"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("aviso: .env não carregado: %v", err)
	}

	cfg := app.LoadConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := app.NewSheetsClient(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}

	sheetNames := configuredGradeSheets(cfg)
	comments, err := client.LoadSheetFeedbacks(ctx, sheetNames)
	if err != nil {
		log.Fatal(err)
	}
	if len(comments) == 0 {
		fmt.Println("Nenhum comentário encontrado nas abas configuradas.")
		return
	}

	printed := false
	for _, sheetName := range sheetNames {
		sheetComments := comments[sheetName]
		if len(sheetComments) == 0 {
			continue
		}
		printed = true
		fmt.Printf("Aba: %s\n", sheetName)
		for _, comment := range sheetComments {
			fmt.Printf("  Célula: %s\n", comment.Cell)
			fmt.Printf("  Autor: %s\n", emptyFallback(comment.Author, "(sem autor)"))
			fmt.Printf("  Texto: %s\n\n", emptyFallback(comment.Text, "(sem texto)"))
		}
	}
	if !printed {
		fmt.Println("Nenhum feedback encontrado nas abas configuradas pela service account.")
	}
}

func configuredGradeSheets(cfg app.Config) []string {
	seen := map[string]bool{}
	var sheetNames []string
	for _, table := range append(cfg.AB1Tables, cfg.AB2Tables...) {
		sheetName := strings.TrimSpace(table.SheetName)
		if sheetName == "" || seen[sheetName] {
			continue
		}
		seen[sheetName] = true
		sheetNames = append(sheetNames, sheetName)
	}
	return sheetNames
}

func emptyFallback(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
