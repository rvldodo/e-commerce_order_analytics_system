package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"math/big"

	"github.com/google/uuid"
)

const claimCodeAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func NewUUID() string {
	return uuid.New().String()
}

func EncryptSHA256(plainText string) string {
	hash := sha256.New()
	hash.Write([]byte(plainText))
	return hex.EncodeToString(hash.Sum(nil))
}

func CompareHash(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func GenerateSecret(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func GenerateClaimCode(n int) (string, error) {
	limit := big.NewInt(int64(len(claimCodeAlphabet)))

	out := make([]byte, n)
	for i := range out {
		idx, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		out[i] = claimCodeAlphabet[idx.Int64()]
	}

	return string(out), nil
}
