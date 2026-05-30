package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"feedback/pkg/app"

	"github.com/joho/godotenv"
)

func main() {
	matricula := flag.String("matricula", "", "matrícula para testar comentários renderizados no payload de notas")
	exam := flag.String("exam", "ab1", "prova/etapa usada com -matricula")
	sheet := flag.String("sheet", "", "aba específica usada com -matricula")
	rawDrive := flag.Bool("raw-drive", false, "imprime comentários brutos retornados pela Google Drive API")
	flag.Parse()

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

	if *rawDrive {
		if err := printRawDriveComments(ctx, client); err != nil {
			log.Fatal(err)
		}
		return
	}

	if strings.TrimSpace(*matricula) != "" {
		if err := printStudentFeedbacks(ctx, client, *matricula, *exam, *sheet); err != nil {
			log.Fatal(err)
		}
		return
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

func printRawDriveComments(ctx context.Context, client *app.SheetsClient) error {
	comments, err := client.LoadDriveCommentDebug(ctx)
	if err != nil {
		return err
	}
	for idx, comment := range comments {
		fmt.Printf("#%d\n", idx+1)
		fmt.Printf("  SheetID: %d (tem sheetId: %t)\n", comment.SheetID, comment.HasSheetID)
		fmt.Printf("  Anchor: %s\n", emptyFallback(comment.Anchor, "(sem anchor)"))
		fmt.Printf("  Quoted: %s\n", emptyFallback(comment.QuotedText, "(sem trecho)"))
		fmt.Printf("  Autor: %s\n", emptyFallback(comment.Author, "(sem autor)"))
		fmt.Printf("  Texto: %s\n\n", emptyFallback(comment.Text, "(sem texto)"))
	}
	fmt.Printf("Comentários brutos do Drive: %d\n", len(comments))
	return nil
}

func printStudentFeedbacks(ctx context.Context, client *app.SheetsClient, matricula string, exam string, sheet string) error {
	identity, err := client.LoginIdentity(ctx, matricula)
	if err != nil {
		return err
	}
	result, err := client.GradeFor(ctx, exam, app.SessionUser{Matricula: identity.Matricula, Name: identity.Name})
	if err != nil {
		return err
	}

	targetSheet := strings.TrimSpace(sheet)
	total := 0
	for _, table := range result.Tables {
		if targetSheet != "" && table.SheetName != targetSheet && table.Label != targetSheet && table.Key != targetSheet {
			continue
		}
		tableCount := 0
		fmt.Printf("Aba: %s\n", table.SheetName)
		for _, card := range table.Cards {
			if strings.TrimSpace(card.Comment) != "" {
				tableCount++
				total++
				fmt.Printf("  Card: %s\n", card.Label)
				fmt.Printf("  Autor: %s\n", emptyFallback(card.CommentAuthor, "(sem autor)"))
				fmt.Printf("  Texto: %s\n\n", card.Comment)
			}
			for _, detail := range card.Details {
				if strings.TrimSpace(detail.Comment) == "" {
					continue
				}
				tableCount++
				total++
				fmt.Printf("  Critério: %s\n", detail.Label)
				fmt.Printf("  Nota: %s\n", detail.DisplayScore)
				fmt.Printf("  Autor: %s\n", emptyFallback(detail.CommentAuthor, "(sem autor)"))
				fmt.Printf("  Texto: %s\n\n", detail.Comment)
			}
		}
		if tableCount == 0 {
			fmt.Println("  Nenhum comentário renderizado nessa aba.")
		}
	}
	if total == 0 {
		if targetSheet != "" {
			return fmt.Errorf("nenhum comentário renderizado para a matrícula %s em %s/%s", matricula, exam, targetSheet)
		}
		return fmt.Errorf("nenhum comentário renderizado para a matrícula %s em %s", matricula, exam)
	}
	fmt.Printf("Comentários renderizados: %d\n", total)
	return nil
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
