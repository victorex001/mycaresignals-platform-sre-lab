package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"

	"therapeutic-engine/internal/db"
	"therapeutic-engine/internal/handlers"
	"therapeutic-engine/internal/router"
	"therapeutic-engine/internal/security"
	"therapeutic-engine/internal/storage"
)

func main() {
	// Goal creation is the first protected route. Startup must therefore have a
	// recognized, non-none authentication mode before the router is created.
	if err := security.ValidateProtectedAuthenticationModeFromEnv(); err != nil {
		log.Fatalf("authentication configuration error: %v", err)
	}
	if err := security.ValidateDormantRoutePolicyFromEnv(); err != nil {
		log.Fatalf("dormant route configuration error: %v", err)
	}

	// Connect to Postgres
	database, err := db.New()
	if err != nil {
		log.Fatalf("db connect error: %v", err)
	}
	defer database.SQL.Close()

	// Stores
	childrenStore := storage.NewChildrenStore(database.SQL)

	// Shared Repo for goals / targets / sessions / timeline
	repo := storage.NewRepo(database.SQL)
	authenticator, err := security.NewAuthenticatorFromEnv(repo)
	if err != nil {
		log.Fatalf("authentication setup error: %v", err)
	}
	awsConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("aws configuration error: %v", err)
	}

	// Handlers
	childrenHandler := &handlers.ChildrenHandler{
		Children: childrenStore,
		Repo:     repo,
	}

	goalsHandler := handlers.NewGoalsHandler(repo)
	riskHandler := handlers.NewRiskHandler(repo)
	carePlanHandler := handlers.NewCarePlanHandler(repo)
	careOverviewHandler := handlers.NewCareOverviewHandler(repo)
	externalFeedbackHandler := handlers.NewExternalFeedbackHandler(repo)
	dailyProgressHandler := handlers.NewDailyProgressHandler(repo)
	demoStakeholderHandler := handlers.NewDemoStakeholderHandler(repo, cognitoidentityprovider.NewFromConfig(awsConfig))
	missingFromHomePlanHandler := handlers.NewMissingFromHomePlanHandler(repo)
	localDevelopmentAccessHandler := handlers.NewLocalDevelopmentAccessHandler(repo)

	therapeuticHandler := handlers.NewTherapeuticHandler(repo)

	// Router
	r := router.New(router.Deps{
		ChildrenHandler:         childrenHandler,
		GoalsHandler:            goalsHandler,
		RiskHandler:             riskHandler,
		CarePlanHandler:         carePlanHandler,
		CareOverviewHandler:     careOverviewHandler,
		ExternalFeedbackHandler: externalFeedbackHandler,
		DailyProgressHandler:    dailyProgressHandler,
		DemoStakeholderHandler:  demoStakeholderHandler,
		MissingFromHomeHandler:  missingFromHomePlanHandler,
		LocalDevelopmentAccess:  localDevelopmentAccessHandler,
		TherapeuticHandler:      therapeuticHandler,
		Authenticator:           authenticator,
		AuthorizationStore:      repo,
		FeedbackIntakeToken:     os.Getenv("FEEDBACK_INTAKE_SERVICE_TOKEN"),
	})

	// Port (default 8081, override with PORT env)
	addr := ":8081"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("therapeutic-engine API listening on %s\n", addr)
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- srv.ListenAndServe()
	}()

	shutdownSignal, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-shutdownSignal.Done():
		log.Printf("shutdown signal received")
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
		return
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownContext); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
		if closeErr := srv.Close(); closeErr != nil {
			log.Printf("forced server close failed: %v", closeErr)
		}
	}
}
