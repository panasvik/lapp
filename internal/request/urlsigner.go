package request

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

type Signer struct {
	secretKey []byte
}

func NewSigner() *Signer {
	return &Signer{secretKey: generateHMACSecret(32)}
}

func (s *Signer) GenerateSignature(path string, userID int, expiresAt int64) string {
	mac := hmac.New(sha256.New, s.secretKey)
	message := fmt.Sprintf("%s:%d:%d", path, userID, expiresAt)
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *Signer) SignURL(path string, userID int, ttl time.Duration) string {
	exp := time.Now().Add(ttl).Unix()
	sig := s.GenerateSignature(path, userID, exp)
	return fmt.Sprintf("%s?exp=%d&uid=%d&sig=%s", path, exp, userID, sig)
}

// Validate проверяет валидность срока действия и подписи
func (s *Signer) Validate(path string, expStr, uidStr, sig string) (int, bool) {
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return 0, false // Истек срок действия
	}

	uid, err := strconv.Atoi(uidStr)
	if err != nil {
		return 0, false
	}

	expectedSig := s.GenerateSignature(path, uid, exp)
	// Защита от timing attacks
	if !hmac.Equal([]byte(expectedSig), []byte(sig)) {
		return 0, false
	}

	return uid, true
}
