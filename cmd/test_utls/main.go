package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"ai-go-browser/browser"
)

func main() {
	tr := browser.NewSafeTransport(false)
	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Println("Iniciando GET para aliexpress.com...")
	req, err := http.NewRequestWithContext(ctx, "GET", "https://aliexpress.com/", nil)
	if err != nil {
		fmt.Printf("Erro req: %v\n", err)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Erro no Do: %v\n", err)
		return
	}
	defer resp.Body.Close()
	fmt.Printf("Status: %d, lendo body...\n", resp.StatusCode)
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Erro no ReadAll: %v\n", err)
		return
	}
	fmt.Printf("Sucesso! Bytes: %d\n", len(b))
}
