package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	_ "github.com/smartraysam/livestream-service/docs"
	"github.com/smartraysam/livestream-service/internal/chat"
	"github.com/smartraysam/livestream-service/internal/config"
	"github.com/smartraysam/livestream-service/internal/db"
	"github.com/smartraysam/livestream-service/internal/events"
	"github.com/smartraysam/livestream-service/internal/middleware"
	"github.com/smartraysam/livestream-service/internal/payment"
	"github.com/smartraysam/livestream-service/internal/recording"
	"github.com/smartraysam/livestream-service/internal/session"
	"github.com/smartraysam/livestream-service/internal/stage"
	"github.com/smartraysam/livestream-service/internal/stream"
	"github.com/smartraysam/livestream-service/internal/ticket"
	"github.com/smartraysam/livestream-service/pkg/laravel"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title           Livestream Service API
// @version         1.0
// @description     Live streaming microservice - supports broadcast (1-to-many) and private (1-to-1) streams, live chat, tipping, and ticketed access.
// @host            localhost:8080
// @BasePath        /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	zerolog.TimeFieldFormat = time.RFC3339
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	logger := log.With().Str("service", "livestream").Logger()
	store := db.NewStore(cfg.DynamoTableStreams, cfg.DynamoTableChat, cfg.DynamoTableTickets)
	if !cfg.UseMemoryStore {
		store, err = db.NewStoreWithDynamo(context.Background(), cfg.AWSRegion, cfg.AWSEndpointURL, cfg.DynamoTableStreams, cfg.DynamoTableChat, cfg.DynamoTableTickets)
	}
	if err != nil {
		logger.Warn().Err(err).Msg("failed to initialize DynamoDB store, using in-memory fallback")
		store = db.NewStore(cfg.DynamoTableStreams, cfg.DynamoTableChat, cfg.DynamoTableTickets)
	} else if cfg.UseMemoryStore {
		logger.Info().Msg("USE_MEMORY_STORE enabled, skipping DynamoDB client")
	}
	laravelClient := laravel.New(cfg.LaravelInternalURL, cfg.LaravelInternalSecret)
	authMW := middleware.NewAuth(laravelClient, cfg.EnableAuth)
	var ivs stream.IVSClient = &stream.MockIVS{}
	if !cfg.UseMockIVS {
		awsIVS, err := stream.NewAWSIVS(context.Background(), cfg.AWSRegion)
		if err != nil {
			logger.Warn().Err(err).Msg("failed to initialize AWS IVS client, using mock")
		} else {
			ivs = awsIVS
		}
	} else {
		logger.Info().Msg("USE_MOCK_IVS enabled, skipping AWS IVS client")
	}

	// IVS Real-Time (Stages) client
	var ivsRT stage.IVSRealTimeClient = &stage.MockIVSRealTime{}
	if !cfg.UseMockIVS {
		awsRT, rtErr := stage.NewAWSIVSRealTime(context.Background(), cfg.AWSRegion)
		if rtErr != nil {
			logger.Warn().Err(rtErr).Msg("failed to initialize AWS IVS Real-Time client, using mock")
		} else {
			ivsRT = awsRT
		}
	}

	var eventPublisher events.Publisher = events.NoopPublisher{}
	if cfg.SQSQueueURL != "" {
		sqsPublisher, err := events.NewSQSPublisher(context.Background(), cfg.AWSRegion, cfg.AWSEndpointURL, cfg.SQSQueueURL)
		if err != nil {
			logger.Warn().Err(err).Msg("failed to initialize SQS publisher, events disabled")
		} else {
			eventPublisher = sqsPublisher
		}
	}

	streamSvc := stream.NewService(store, ivs)
	hubs := &chat.HubManager{}
	sessionSvc := session.NewService(store, streamSvc, laravelClient, eventPublisher)
	paymentSvc := payment.NewService(store, streamSvc, laravelClient, hubs, eventPublisher)
	ticketSvc := ticket.NewService(store, streamSvc, laravelClient, eventPublisher)
	recordingSvc := recording.NewService(store, streamSvc, hubs)

	streamH := stream.NewHandler(streamSvc, store, laravelClient)
	sessionH := session.NewHandler(sessionSvc)
	chatH := chat.NewHandler(store, streamSvc, hubs)
	paymentH := payment.NewHandler(paymentSvc)
	ticketH := ticket.NewHandler(ticketSvc)
	recordingH := recording.NewHandler(cfg, recordingSvc, streamSvc, ticketSvc)
	stageStore := stage.NewMemStore()
	stageSvc := stage.NewService(stageStore, ivsRT)
	stageH := stage.NewHandler(stageSvc)

	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(middleware.Logger(logger))
	r.Mount("/docs", httpSwagger.WrapHandler)

	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/streams", streamH.ListStreams)
		api.Get("/streams/{id}", streamH.GetStream)
		api.Get("/streams/creator/{creator_id}", streamH.ListByCreator)
		api.Post("/webhooks/ivs", recordingH.IVSWebhook)

		api.Group(func(protected chi.Router) {
			protected.Use(authMW.Require)
			protected.Post("/streams", streamH.CreateStream)
			protected.Post("/streams/{id}/start-live", streamH.StartLiveBroadcast)
			protected.Post("/streams/{id}/stop-live", streamH.StopLiveBroadcast)
			protected.Get("/streams/{id}/ingest-info", streamH.GetIngestInfo)
			protected.Get("/streams/{id}/ivs-status", streamH.GetIVSStatus)
			protected.Get("/streams/{id}/playback", streamH.GetPlayback)
			protected.Get("/streams/{id}/access", streamH.AccessCheck)
			protected.Post("/streams/{id}/sync", streamH.SyncToLaravel)
			protected.Patch("/streams/{id}", streamH.UpdateStream)
			protected.Delete("/streams/{id}", streamH.DeleteStream)
			protected.Get("/streams/{id}/chat", chatH.Connect)
			protected.Get("/streams/{id}/chat/history", chatH.History)
			protected.Post("/streams/{id}/tip", paymentH.TipStream)
			protected.Post("/streams/{id}/ticket/purchase", ticketH.PurchaseTicket)
			protected.Get("/streams/{id}/ticket/verify", ticketH.VerifyTicket)
			protected.Get("/streams/{id}/recording", recordingH.GetRecording)

			protected.Post("/sessions", sessionH.CreateSession)
			protected.Post("/sessions/{id}/invite", sessionH.InviteViewer)
			protected.Get("/sessions/incoming", sessionH.IncomingInvites)
			protected.Post("/sessions/{id}/accept", sessionH.AcceptInvite)
			protected.Post("/sessions/{id}/decline", sessionH.DeclineInvite)

			// ── IVS Real-Time Stages ──────────────────────────────────────
			// 1-to-1 (CALL) and 1-to-many (BROADCAST) via WebRTC
			protected.Get("/stages", stageH.ListMyStages)
			protected.Post("/stages", stageH.CreateStage)
			protected.Get("/stages/{id}", stageH.GetStage)
			protected.Delete("/stages/{id}", stageH.EndStage)
			protected.Post("/stages/{id}/join", stageH.JoinStage)
			protected.Delete("/stages/{id}/participants/{pid}", stageH.DisconnectParticipant)
		})
	})

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}
	go func() {
		logger.Info().Str("addr", srv.Addr).Msg("server started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	logger.Info().Msg("server gracefully stopped")
}
