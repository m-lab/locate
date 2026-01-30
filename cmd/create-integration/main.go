package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/m-lab/go/flagx"
	"github.com/m-lab/go/rtx"
)

const (
	integrationMetaKind   = "IntegrationMeta"
	integrationAPIKeyKind = "IntegrationAPIKey"
	apiKeyStatusActive    = "active"
	keySecretBytes        = 32
)

// integrationAPIKey represents an API key entity in Datastore.
type integrationAPIKey struct {
	KeyHash     string    `datastore:"key_hash"`
	CreatedAt   time.Time `datastore:"created_at"`
	Description string    `datastore:"description"`
	Status      string    `datastore:"status"`
}

var (
	project       string
	namespace     string
	integrationID string
	keyID         string
	description   string
)

func init() {
	flag.StringVar(&project, "project", "", "GCP project ID (required)")
	flag.StringVar(&namespace, "namespace", "client-integration", "Datastore namespace")
	flag.StringVar(&integrationID, "integration-id", "", "Integration ID (required)")
	flag.StringVar(&keyID, "key-id", "", "Key ID (optional, auto-generated if not provided)")
	flag.StringVar(&description, "description", "", "Human-readable description (optional)")
}

func main() {
	flag.Parse()
	rtx.Must(flagx.ArgsFromEnvWithLog(flag.CommandLine, false), "failed to read args from env")

	// Validate required flags.
	if project == "" {
		log.Fatal("--project is required")
	}
	if integrationID == "" {
		log.Fatal("--integration-id is required")
	}

	// Generate key ID if not provided.
	if keyID == "" {
		keyID = generateKeyID()
	}

	// Generate random key secret (32 bytes, base64url encoded).
	keySecret, err := generateKeySecret()
	rtx.Must(err, "failed to generate key secret")

	// Compute SHA-256 hash of the key secret.
	keyHash := computeHash(keySecret)

	// Create Datastore client.
	ctx := context.Background()
	client, err := datastore.NewClient(ctx, project)
	rtx.Must(err, "failed to create Datastore client")
	defer client.Close()

	// Create the API key entity.
	apiKeyEntity := &integrationAPIKey{
		KeyHash:     keyHash,
		CreatedAt:   time.Now().UTC(),
		Description: description,
		Status:      apiKeyStatusActive,
	}

	// Create hierarchical key structure:
	// IntegrationMeta (parent) -> IntegrationAPIKey (child)
	parentKey := datastore.NameKey(integrationMetaKind, integrationID, nil)
	childKey := datastore.NameKey(integrationAPIKeyKind, keyID, parentKey)

	if namespace != "" {
		parentKey.Namespace = namespace
		childKey.Namespace = namespace
	}

	// First, ensure the parent entity exists (upsert with empty struct).
	_, err = client.Put(ctx, parentKey, &struct{}{})
	rtx.Must(err, "failed to create parent IntegrationMeta entity")

	// Then create the child API key entity.
	_, err = client.Put(ctx, childKey, apiKeyEntity)
	rtx.Must(err, "failed to create IntegrationAPIKey entity")

	// Construct the full API key.
	apiKey := fmt.Sprintf("mlabk.cii_%s.ki_%s.%s", integrationID, keyID, keySecret)

	// Output the result.
	fmt.Println("Created integration API key:")
	fmt.Printf("  Integration ID: %s\n", integrationID)
	fmt.Printf("  Key ID: %s\n", keyID)
	fmt.Printf("  API Key: %s\n", apiKey)
	fmt.Println()
	fmt.Println("Store this API key securely - the secret cannot be recovered!")
}

// generateKeyID generates a random key ID with format "k_<random>".
func generateKeyID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("failed to generate random key ID: %v", err)
	}
	return "k_" + hex.EncodeToString(b)
}

// generateKeySecret generates a random key secret (32 bytes, base64url encoded).
func generateKeySecret() (string, error) {
	b := make([]byte, keySecretBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// computeHash computes the SHA-256 hash of the given secret and returns it as hex.
func computeHash(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}

// printUsage prints usage information.
func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "Options:")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr, "\nExample:")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/create-integration/main.go \\")
	fmt.Fprintln(os.Stderr, "    --project=mlab-sandbox \\")
	fmt.Fprintln(os.Stderr, "    --integration-id=test-client \\")
	fmt.Fprintln(os.Stderr, "    --description=\"Test credential for e2e testing\"")
}
