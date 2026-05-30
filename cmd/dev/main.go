package main

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"strings"

	handler "feedback/api"
)

func main() {
	loadDotEnv(".env")

	static := http.FileServer(http.Dir("dist"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			handler.Handler(w, r)
			return
		}
		if _, err := os.Stat("dist" + r.URL.Path); err == nil && r.URL.Path != "/" {
			static.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, "dist/index.html")
	})

	log.Println("dev server em http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if os.Getenv(strings.TrimSpace(key)) == "" {
			os.Setenv(strings.TrimSpace(key), strings.TrimSpace(value))
		}
	}
}
