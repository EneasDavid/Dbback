package main

import (
	"bufio"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	handler "feedback/api"
)

func main() {
	loadDotEnv(".env")
	os.Setenv("COOKIE_SECURE", "false")

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
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "GOOGLE_SERVICE_ACCOUNT_JSON" && strings.HasPrefix(value, "{") && !json.Valid([]byte(value)) {
			for scanner.Scan() {
				value += "\n" + scanner.Text()
				if json.Valid([]byte(value)) {
					break
				}
			}
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}
