// did.go — DID Document endpoint (/.well-known/did.json).
//
// Serves the issuer's DID Document so verifiers can resolve did:web:<domain>
// and retrieve the Ed25519 public key for signature verification.
package handler

import (
	"github.com/gofiber/fiber/v2"
	"the-badge/internal/service"
	"the-badge/pkg/response"
)

// DIDDocument handles GET /.well-known/did.json.
//
// It reads the SignerService from Fiber locals (key "signer") and constructs
// a W3C DID Core document containing the issuer's Ed25519 public key in
// Multikey format.
//
// The DID document structure follows:
//
//	{
//	    "@context": ["https://www.w3.org/ns/did/v1", "https://w3id.org/security/multikey/v1"],
//	    "id": "did:web:<domain>",
//	    "verificationMethod": [{
//	        "id": "did:web:<domain>#key-1",
//	        "type": "Multikey",
//	        "controller": "did:web:<domain>",
//	        "publicKeyMultibase": "z<base58btc>"
//	    }],
//	    "assertionMethod": ["did:web:<domain>#key-1"]
//	}
func DIDDocument(c *fiber.Ctx) error {
	signer, ok := c.Locals("signer").(*service.SignerService)
	if !ok || signer == nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Signer service not available")
	}

	issuerDID, _ := c.Locals("issuerDID").(string)
	if issuerDID == "" {
		issuerDID = "did:web:localhost"
	}

	keyID := signer.KeyID()

	doc := service.DIDDocument{
		Context: []string{
			"https://www.w3.org/ns/did/v1",
			"https://w3id.org/security/multikey/v1",
		},
		ID: issuerDID,
		VerificationMethod: []service.VerificationMethod{
			{
				ID:                 keyID,
				Type:               "Multikey",
				Controller:         issuerDID,
				PublicKeyMultibase: signer.PublicKeyMultibase(),
			},
		},
		AssertionMethod: []string{keyID},
	}

	// Add Polygon fallback service if contract addresses are configured.
	keyRegAddr, _ := c.Locals("keyRegistryAddress").(string)
	badgeRegAddr, _ := c.Locals("badgeRegistryAddress").(string)
	polygonNetwork, _ := c.Locals("polygonNetwork").(string)
	if polygonNetwork == "" {
		polygonNetwork = "polygon-mainnet"
	}
	if keyRegAddr != "" && badgeRegAddr != "" {
		doc.Service = []service.DIDService{
			{
				ID:   issuerDID + "#polygon-fallback",
				Type: "BlockchainKeyRegistry",
				ServiceEndpoint: map[string]string{
					"network":              polygonNetwork,
					"keyRegistryAddress":   keyRegAddr,
					"badgeRegistryAddress": badgeRegAddr,
					"description":          "이 도메인이 사라져도 위 컨트랙트에서 공개키 이력과 배지 상태를 조회할 수 있습니다",
				},
			},
		}
	}

	return c.JSON(doc)
}
