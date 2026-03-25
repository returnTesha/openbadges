// Package main is the entry point for The Badge server.
// It bootstraps the Fiber application, registers middleware and routes, starts
// the goroutine worker pool, and handles graceful shutdown via OS signals.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"the-badge/internal/config"
	"the-badge/internal/handler"
	"the-badge/internal/middleware"
	"the-badge/internal/model"
	"the-badge/internal/repository"
	"the-badge/internal/router"
	"the-badge/internal/service"
	"the-badge/internal/worker"
	"the-badge/pkg/blockchain"
	"the-badge/pkg/response"
)

func main() {
	// ── Configuration ───────────────────────────────────────────────────
	cfg := config.Load()

	// ── PostgreSQL connection pool ──────────────────────────────────────
	ctx := context.Background()
	dbPool, err := initDB(ctx, cfg)
	if err != nil {
		log.Fatalf("[server] failed to connect to database: %v", err)
	}
	defer dbPool.Close()
	log.Println("[server] database connected")

	// Run migration hint.
	log.Println("[server] NOTE: ensure DB schema is applied — run: psql $DATABASE_URL < migrations/001_initial_schema.sql")

	// ── Repository ──────────────────────────────────────────────────────
	repo := repository.NewPostgresRepository(dbPool)

	// ── Blockchain client ───────────────────────────────────────────────
	bcClient := initBlockchain(ctx, cfg)

	// ── Signer service ─────────────────────────────────────────────────
	signer, err := initSigner(cfg)
	if err != nil {
		log.Fatalf("[server] failed to initialize signer: %v", err)
	}
	log.Printf("[server] signer initialized (keyID=%s)", signer.KeyID())

	// ── Ensure issuer row exists in DB ─────────────────────────────────
	ensureIssuer(ctx, dbPool, cfg, signer)

	// ── Credential builder ──────────────────────────────────────────────
	credBuilder := service.NewCredentialBuilder(signer)

	// ── DID resolver & verification service ─────────────────────────────
	// Use KEY_REGISTRY_ADDRESS as the didFallback contract address.
	contractAddr := cfg.KeyRegistryAddress
	if contractAddr == "" {
		contractAddr = cfg.ContractAddress
	}

	var keyReader service.KeyRegistryReader
	var revChecker service.RevocationChecker
	if bcClient != nil {
		keyReader = bcClient
		revChecker = bcClient
	}
	resolver := service.NewDIDResolver(cfg.PolygonRPC, contractAddr, keyReader)

	// Configure local DID document so the resolver can verify our own badges
	// without making an HTTP request to did:web (essential for local dev).
	localDoc := &service.DIDDocument{
		Context: []string{
			"https://www.w3.org/ns/did/v1",
			"https://w3id.org/security/multikey/v1",
		},
		ID: cfg.IssuerDID,
		VerificationMethod: []service.VerificationMethod{
			{
				ID:                 signer.KeyID(),
				Type:               "Multikey",
				Controller:         cfg.IssuerDID,
				PublicKeyMultibase: signer.PublicKeyMultibase(),
			},
		},
		AssertionMethod: []string{signer.KeyID()},
	}
	if cfg.KeyRegistryAddress != "" && cfg.BadgeRegistryAddress != "" {
		localDoc.Service = []service.DIDService{
			{
				ID:   cfg.IssuerDID + "#polygon-fallback",
				Type: "BlockchainKeyRegistry",
				ServiceEndpoint: map[string]string{
					"network":              cfg.PolygonNetwork,
					"keyRegistryAddress":   cfg.KeyRegistryAddress,
					"badgeRegistryAddress": cfg.BadgeRegistryAddress,
					"description":          "이 도메인이 사라져도 위 컨트랙트에서 공개키 이력과 배지 상태를 조회할 수 있습니다",
				},
			},
		}
	}
	resolver.SetLocalDIDDocument(localDoc)
	log.Printf("[server] local DID document configured for %s", cfg.IssuerDID)

	verifier := service.NewVerificationService(resolver, revChecker)

	// ── Worker pool ─────────────────────────────────────────────────────
	pool := worker.NewPool(cfg.WorkerCount, cfg.JobQueueSize)

	// Register processors for both job types.
	pool.RegisterProcessor(worker.JobTypeIssue, makeIssueProcessor(credBuilder, cfg, bcClient, repo, dbPool))
	pool.RegisterProcessor(worker.JobTypeVerify, makeVerifyProcessor(verifier, repo))

	pool.Start()

	// ── Fiber app ───────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		Prefork:      false,
		AppName:      "the-badge",
		ServerHeader: "the-badge",
		// Structured error handler — returns RFC 7807 for unhandled errors.
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return response.Error(c, code, "Internal Server Error", err.Error())
		},
	})

	// ── Middleware (order matters) ───────────────────────────────────────
	// 1. Request logging — always first so every request is logged.
	app.Use(middleware.Logger())

	// 2. Rate limiting — applied globally.
	app.Use(middleware.RateLimit(100))

	// 3. Auth — MVP bypass (all requests pass through).
	app.Use(middleware.Auth(
		"/health",
		"/.well-known/did.json",
		"/api/v1/badges/verify",
		"/api/v1/badges/",
	))

	// 4. Inject shared services and config into Fiber locals.
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("pool", pool)
		c.Locals("signer", signer)
		c.Locals("credentialBuilder", credBuilder)
		c.Locals("verifier", verifier)
		c.Locals("repo", repo)
		c.Locals("issuerDID", cfg.IssuerDID)
		c.Locals("issuerName", cfg.IssuerName)
		c.Locals("serverURL", cfg.ServerURL)
		c.Locals("contractAddress", contractAddr)
		c.Locals("keyRegistryAddress", cfg.KeyRegistryAddress)
		c.Locals("badgeRegistryAddress", cfg.BadgeRegistryAddress)
		c.Locals("polygonNetwork", cfg.PolygonNetwork)
		if bcClient != nil {
			c.Locals("blockchain", bcClient)
		}
		return c.Next()
	})

	// ── Routes ──────────────────────────────────────────────────────────
	router.Setup(app)

	// ── Graceful shutdown ───────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("[server] shutting down...")
		pool.Shutdown()
		if bcClient != nil {
			bcClient.Close()
		}
		dbPool.Close()
		if err := app.Shutdown(); err != nil {
			log.Printf("[server] shutdown error: %v", err)
		}
	}()

	// ── Start listening ─────────────────────────────────────────────────
	addr := ":" + cfg.ServerPort
	log.Printf("[server] listening on %s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("[server] listen error: %v", err)
	}
}

// ── Initialization helpers ──────────────────────────────────────────────────

// initDB creates a pgxpool connection pool from config.
func initDB(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	dsn := cfg.DSN()
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid database DSN: %w", err)
	}
	poolCfg.MaxConns = 20

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connectivity.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return pool, nil
}

// initBlockchain creates a blockchain client if the required config is present.
// Returns nil (not an error) if addresses are not configured.
func initBlockchain(ctx context.Context, cfg *config.Config) *blockchain.Client {
	if cfg.KeyRegistryAddress == "" || cfg.BadgeRegistryAddress == "" || cfg.PolygonRPC == "" {
		log.Println("[server] blockchain client disabled (KEY_REGISTRY_ADDRESS or BADGE_REGISTRY_ADDRESS not set)")
		return nil
	}

	client, err := blockchain.NewClient(ctx,
		cfg.PolygonRPC,
		cfg.KeyRegistryAddress,
		cfg.BadgeRegistryAddress,
		cfg.DeployerPrivateKey,
	)
	if err != nil {
		log.Printf("[server] WARNING: blockchain client init failed: %v (continuing without blockchain)", err)
		return nil
	}

	log.Printf("[server] blockchain client connected (keyReg=%s, badgeReg=%s)",
		cfg.KeyRegistryAddress, cfg.BadgeRegistryAddress)
	return client
}

// initSigner creates a SignerService from the config. If no private key is
// configured (local dev), it generates a throwaway key pair automatically.
func initSigner(cfg *config.Config) (*service.SignerService, error) {
	keyID := cfg.IssuerDID + "#key-1"

	if cfg.Ed25519PrivateKeyHex != "" {
		privBytes, err := hex.DecodeString(cfg.Ed25519PrivateKeyHex)
		if err != nil {
			return nil, fmt.Errorf("invalid ED25519_PRIVATE_KEY_HEX: %w", err)
		}
		// If only the 32-byte seed was provided, expand to full 64-byte private key.
		if len(privBytes) == ed25519.SeedSize {
			privBytes = ed25519.NewKeyFromSeed(privBytes)
		}
		return service.NewSignerService(privBytes, keyID)
	}

	// No key configured — generate an ephemeral dev key pair.
	log.Println("[server] WARNING: No ED25519_PRIVATE_KEY_HEX configured; generating ephemeral dev key pair")
	_, priv, err := service.GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	return service.NewSignerService(priv, keyID)
}

// ensureIssuer creates or updates the issuer row in the database so that
// badge saves don't silently fail on a fresh DB. Uses raw SQL to avoid
// modifying the repository package (DBA domain).
func ensureIssuer(ctx context.Context, dbPool *pgxpool.Pool, cfg *config.Config, signer *service.SignerService) {
	const upsertSQL = `
		INSERT INTO issuers (did, name, url, public_key)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (did) DO UPDATE SET
			name       = EXCLUDED.name,
			url        = EXCLUDED.url,
			public_key = EXCLUDED.public_key,
			updated_at = now()`

	_, err := dbPool.Exec(ctx, upsertSQL,
		cfg.IssuerDID,
		cfg.IssuerName,
		cfg.ServerURL,
		signer.PublicKeyBytes(),
	)
	if err != nil {
		log.Printf("[server] WARNING: failed to ensure issuer row: %v (badge saves may fail)", err)
		return
	}
	log.Printf("[server] issuer ensured in DB: did=%s name=%s", cfg.IssuerDID, cfg.IssuerName)
}

// ── Worker processors ───────────────────────────────────────────────────────

// makeVerifyProcessor returns a worker.ProcessFunc that verifies a badge
// credential and saves a verification log to the database.
func makeVerifyProcessor(verifier *service.VerificationService, repo *repository.PostgresRepository) worker.ProcessFunc {
	return func(job worker.Job) worker.JobResult {
		credJSON, ok := job.Payload.([]byte)
		if !ok {
			return worker.JobResult{
				JobID: job.ID,
				Error: fmt.Errorf("verify processor: unexpected payload type %T", job.Payload),
			}
		}

		ctx := context.Background()
		result, err := verifier.VerifyCredential(ctx, credJSON)
		if err != nil {
			return worker.JobResult{JobID: job.ID, Error: fmt.Errorf("verify processor: %w", err)}
		}

		// Save verification log to DB (best-effort).
		saveVerificationLog(ctx, repo, result)

		return worker.JobResult{JobID: job.ID, Data: result}
	}
}

// makeIssueProcessor returns a worker.ProcessFunc that builds a signed
// credential, saves it to the database, and records the hash on-chain.
func makeIssueProcessor(builder *service.CredentialBuilder, cfg *config.Config, bcClient *blockchain.Client, repo *repository.PostgresRepository, dbPool *pgxpool.Pool) worker.ProcessFunc {
	// Use KEY_REGISTRY_ADDRESS as contractAddress for badge JSON if available.
	contractAddr := cfg.KeyRegistryAddress
	if contractAddr == "" {
		contractAddr = cfg.ContractAddress
	}

	return func(job worker.Job) worker.JobResult {
		req, ok := job.Payload.(*handler.IssueBadgeRequest)
		if !ok {
			return worker.JobResult{
				JobID: job.ID,
				Error: fmt.Errorf("issue processor: unexpected payload type %T", job.Payload),
			}
		}

		// Generate credential_id as {CODE}-{CATEGORY}-{year}{seq}.
		ctx := context.Background()
		seq, err := repo.NextCredentialSequence(ctx)
		if err != nil {
			return worker.JobResult{JobID: job.ID, Error: fmt.Errorf("issue processor: credential sequence: %w", err)}
		}
		credentialID := fmt.Sprintf("%s-%s-%d%d", req.UniversityCode, req.ProgramCategory, time.Now().Year(), seq)

		// Build recipient DID: prefer student_id, fall back to credential_id.
		recipientDID := ""
		if req.StudentID != "" {
			recipientDID = fmt.Sprintf("%s:users:%s", cfg.IssuerDID, req.StudentID)
		} else {
			recipientDID = fmt.Sprintf("%s:users:%s", cfg.IssuerDID, credentialID)
		}

		params := service.CredentialParams{
			IssuerDID:       cfg.IssuerDID,
			IssuerName:      cfg.IssuerName,
			ServerURL:       cfg.ServerURL,
			ContractAddress: contractAddr,

			CredentialID:    credentialID,
			AchievementID:   req.AchievementID,
			AchievementName: req.AchievementName,
			AchievementDesc: req.AchievementDesc,
			Criteria:        req.Criteria,
			ImageBase64:     req.ImageBase64,

			UniversityCode:  req.UniversityCode,
			ProgramID:       req.ProgramID,
			ProgramCategory: req.ProgramCategory,
			StudentID:       req.StudentID,

			RecipientName:  req.RecipientName,
			RecipientEmail: req.RecipientEmail,
			RecipientDID:   recipientDID,
		}

		credential, err := builder.BuildCredential(params)
		if err != nil {
			return worker.JobResult{JobID: job.ID, Error: fmt.Errorf("issue processor: %w", err)}
		}

		// Extract credential metadata.
		jsonBytes, err := json.Marshal(credential)
		if err != nil {
			log.Printf("[issue] WARNING: failed to marshal credential JSON: %v", err)
			return worker.JobResult{JobID: job.ID, Data: credential}
		}
		hash := sha256.Sum256(jsonBytes)

		// Extract proof value for DB storage.
		proofValue := ""
		if proof, ok := credential["proof"].(map[string]interface{}); ok {
			proofValue, _ = proof["proofValue"].(string)
		}

		// Save badge to DB (best-effort — may fail if issuer/achievement not seeded).
		saveBadgeToDB(repo, cfg, req, credentialID, recipientDID, jsonBytes, proofValue, hash[:])

		// Record issuance hash on-chain (non-blocking).
		if bcClient != nil {
			go recordOnChain(bcClient, dbPool, credentialID, hash)
		}

		return worker.JobResult{JobID: job.ID, Data: credential}
	}
}

// saveBadgeToDB persists the issued badge to PostgreSQL.
// Errors are logged but do not fail the issuance — the credential is returned
// to the caller regardless.
func saveBadgeToDB(repo *repository.PostgresRepository, cfg *config.Config, req *handler.IssueBadgeRequest, credentialID string, recipientDID string, credJSON []byte, proofValue string, blockchainHash []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Look up issuer by DID to get the FK UUID.
	issuer, err := repo.GetIssuerByDID(ctx, cfg.IssuerDID)
	if err != nil || issuer == nil {
		log.Printf("[db] WARNING: issuer %q not found in DB, skipping badge save (seed issuers table first)", cfg.IssuerDID)
		return
	}

	// Parse achievement ID if provided (nullable after migration 003).
	// Non-UUID strings (e.g. "snu-leadership-2026") are allowed — just skip
	// the FK link and save the badge without an achievement reference.
	var achievementID *uuid.UUID
	if req.AchievementID != "" {
		parsed, err := uuid.Parse(req.AchievementID)
		if err != nil {
			log.Printf("[db] WARNING: achievement_id %q is not a valid UUID, saving badge without achievement link", req.AchievementID)
		} else {
			achievementID = &parsed
		}
	}

	badge := &model.Badge{
		AchievementID:  achievementID,
		IssuerID:       issuer.ID,
		RecipientName:  req.RecipientName,
		RecipientEmail: req.RecipientEmail,
		RecipientDID:   recipientDID,
		CredentialID:   credentialID,
		CredentialJSON: credJSON,
		ProofValue:     proofValue,
		BlockchainHash: blockchainHash,
		UniversityCode: req.UniversityCode,
		ProgramID:      req.ProgramID,
		StudentID:      req.StudentID,
		Status:         "active",
	}

	if err := repo.CreateBadge(ctx, badge); err != nil {
		log.Printf("[db] WARNING: failed to save badge %s: %v", credentialID, err)
		return
	}

	log.Printf("[db] badge saved: id=%s credential_id=%s", badge.ID, credentialID)
}

// recordOnChain records the credential hash on the BadgeRegistry contract and
// updates the badge's blockchain_tx_hash in the database.
func recordOnChain(bcClient *blockchain.Client, dbPool *pgxpool.Pool, credentialID string, hash [32]byte) {
	if credentialID == "" {
		log.Println("[blockchain] WARNING: empty credential_id, skipping on-chain recording")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	txHash, err := bcClient.RecordIssuance(ctx, credentialID, hash)
	if err != nil {
		log.Printf("[blockchain] WARNING: RecordIssuance failed for %s: %v", credentialID, err)
		return
	}

	log.Printf("[blockchain] recorded issuance hash for %s (tx=%s)", credentialID, txHash)

	// Update the badge's blockchain_tx_hash in the DB (best-effort).
	updateCtx, updateCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer updateCancel()

	_, err = dbPool.Exec(updateCtx,
		"UPDATE badges SET blockchain_tx_hash = $1 WHERE credential_id = $2",
		txHash, credentialID)
	if err != nil {
		log.Printf("[db] WARNING: failed to update blockchain_tx_hash for %s: %v", credentialID, err)
	}
}

// saveVerificationLog persists a verification result to the database.
func saveVerificationLog(ctx context.Context, repo *repository.PostgresRepository, result *service.VerificationResult) {
	logResult := "valid"
	failureReason := ""
	if !result.Valid {
		logResult = "invalid"
		if len(result.Errors) > 0 {
			failureReason = result.Errors[0]
			// Check for specific error types.
			for _, e := range result.Errors {
				if e == "credential has been revoked" {
					logResult = "revoked"
				}
				if fmt.Sprintf("%s", e) == "credential has expired at "+result.ExpiresAt {
					logResult = "expired"
				}
			}
		}
	}

	// Serialize verification errors as detail JSON.
	var detail json.RawMessage
	if len(result.Errors) > 0 {
		detail, _ = json.Marshal(map[string]interface{}{
			"errors": result.Errors,
		})
	}

	// Extract credential ID from the credential map.
	credentialID := ""
	if result.Credential != nil {
		credentialID, _ = result.Credential["id"].(string)
	}

	vlog := &model.VerificationLog{
		CredentialID:  credentialID,
		IssuerDID:     result.IssuerDID,
		Result:        logResult,
		FailureReason: failureReason,
		Detail:        detail,
	}

	saveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := repo.CreateVerificationLog(saveCtx, vlog); err != nil {
		log.Printf("[db] WARNING: failed to save verification log: %v", err)
	}
}

