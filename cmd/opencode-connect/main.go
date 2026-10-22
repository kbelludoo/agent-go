package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/kbelludoo/agent-go/pkg/opencode"
)

func main() {
	var (
		serverURL = flag.String("url", "", "OpenCode server URL (e.g. http://127.0.0.1:28894). If omitted, discovers automatically.")
		password  = flag.String("password", "", "Server password. If omitted, discovers from local service registration or config.")
		channel   = flag.String("channel", "dev", "Service channel to discover (latest, dev, local, etc.)")
		testToken = flag.Bool("test-token", false, "Demonstrate session token signing & verification locally")
	)
	flag.Parse()

	fmt.Println("==================================================")
	fmt.Println("  OpenCode Go Client & Authentication Validator   ")
	fmt.Println("==================================================")

	if *testToken {
		demoTokenSigning()
		return
	}

	targetURL := *serverURL
	targetPassword := *password

	// Auto-discovery if no URL specified
	if targetURL == "" {
		fmt.Printf("[Discovery] Procurando serviço OpenCode no canal '%s'...\n", *channel)
		reg, err := opencode.DiscoverLocalService(*channel)
		if err != nil {
			fmt.Printf("[Aviso] Não foi possível descobrir serviço local automaticamente: %v\n", err)
			stateDir, _ := opencode.DefaultStateDir()
			fmt.Printf("[Dica] Verifique se o OpenCode está rodando ('opencode serve --service') ou forneça --url e --password.\n")
			fmt.Printf("       Caminho esperado do registro: %s\\%s\n", stateDir, opencode.ServiceFilename(*channel))

			fmt.Println("\n[Demo] Executando validação de token e regras de autenticação localmente:")
			demoTokenSigning()
			return
		}

		fmt.Printf("[Discovery] Serviço encontrado!\n")
		fmt.Printf("  - URL:     %s\n", reg.URL)
		fmt.Printf("  - PID:     %d\n", reg.PID)
		fmt.Printf("  - Versão:  %s\n", reg.Version)
		if reg.Password != "" {
			fmt.Printf("  - Senha:   (carregada com sucesso, %d chars)\n", len(reg.Password))
		} else {
			fmt.Printf("  - Senha:   (nenhuma senha configurada / sem autenticação)\n")
		}

		targetURL = reg.URL
		if targetPassword == "" {
			targetPassword = reg.Password
		}
	}

	client := opencode.NewServerClient(targetURL, targetPassword)

	fmt.Printf("\n[Validação] Conectando a %s via GET /api/info...\n", targetURL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := client.Probe(ctx)
	if err != nil {
		fmt.Printf("[ERRO] Falha na conexão ou validação: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("[Sucesso] Servidor validado com sucesso!")
	fmt.Printf("  - Versão:       %s\n", info.Version)
	fmt.Printf("  - PID Remoto:   %d\n", info.PID)
	fmt.Printf("  - URLs ativas:  %v\n", info.URLs)
	fmt.Printf("  - Pasta Temp:   %s\n", info.Paths.Tmp)
	fmt.Printf("  - Capacidades:  %+v\n", info.Capabilities)

	if targetPassword != "" {
		fmt.Println("\n[Token de Sessão] Gerando token de sessão assinado com HMAC-SHA256...")
		now := time.Now()
		sessionToken := opencode.IssueSession(targetPassword, now)
		fmt.Printf("  - Token gerado: %s\n", sessionToken)

		valid := opencode.VerifySession(sessionToken, targetPassword, now)
		fmt.Printf("  - Verificação do token (local): %t\n", valid)

		// Test connecting using session token as password
		tokenClient := opencode.NewServerClient(targetURL, sessionToken)
		_, err := tokenClient.Probe(ctx)
		if err == nil {
			fmt.Println("  - Autenticação no servidor usando o Session Token: SUCESSO!")
		} else {
			fmt.Printf("  - Autenticação com Session Token no servidor: %v\n", err)
		}
	}
}

func demoTokenSigning() {
	pass := "exemplo-senha-secreta-12345"
	now := time.Now()

	fmt.Println("\n1. Derivação e Assinatura de Token OpenCode:")
	fmt.Printf("   Senha base: %s\n", pass)
	token := opencode.IssueSession(pass, now)
	fmt.Printf("   Token de Sessão: %s\n", token)

	isValid := opencode.VerifySession(token, pass, now)
	fmt.Printf("   Validação com senha correta: %t\n", isValid)

	isInvalid := opencode.VerifySession(token, "senha-errada", now)
	fmt.Printf("   Validação com senha incorreta: %t\n", isInvalid)

	fmt.Println("\n2. Cabeçalho HTTP Basic Auth:")
	authHeader := opencode.BasicAuthHeader("opencode", pass)
	fmt.Printf("   Authorization: %s\n", authHeader)

	fmt.Println("\n3. Query Param para WebSockets / SSE:")
	queryVal := opencode.AuthTokenQueryValue("opencode", pass)
	fmt.Printf("   ?auth_token=%s\n", queryVal)

	fmt.Println("\n4. Nome do Cookie de Sessão:")
	cookieName := opencode.SessionCookieName("localhost:28894")
	fmt.Printf("   Cookie para host localhost:28894: %s\n", cookieName)
}
