package jwt

import (
	"e-commerce_order_analytics_system/pkg/crypto"
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
)

type Tokenizer struct {
	Config Config
}

type Config struct {
	AccessTokenSecret  string
	RefreshTokenSecret string
}

type TokenPair struct {
	UUID        string
	SignedToken string
}

type TokenDetail struct {
	AccessTokenUUID           string
	RefreshTokenUUID          string
	AccessToken               string
	RefreshToken              string
	AccessTokenExpiredAt      int64
	RefreshTokenExpiredAt     int64
	AccessTokenExpiredAtTime  time.Time
	RefreshTokenExpiredAtTime time.Time
}

type ClaimsDetail struct {
	Valid            bool
	UserID           int64
	AccessTokenUUID  string
	RefreshTokenUUID string
}

const (
	AccessTokenType  = "access_token"
	RefreshTokenType = "refresh_token"

	expired1Day   = time.Hour * 24
	expired7Days  = time.Hour * 24 * 7
	expired30Days = time.Hour * 24 * 30
)

var tokenizer *Tokenizer

func New(config Config) *Tokenizer {
	tokenizer = &Tokenizer{
		Config: Config{
			AccessTokenSecret:  config.AccessTokenSecret,
			RefreshTokenSecret: config.RefreshTokenSecret,
		},
	}

	return tokenizer
}

type SingleTokenParam struct {
	Type        string
	UserID      int64
	ExpiredTime int64
	Secret      string
}

func generateSingleToken(param SingleTokenParam) (TokenPair, error) {
	tokenPair := TokenPair{}
	uuid := crypto.NewUUID()
	claims := jwt.MapClaims{
		"user_id": param.UserID,
		"exp":     param.ExpiredTime,
	}
	switch param.Type {
	case RefreshTokenType:
		claims["refresh_token_uuid"] = uuid
	case AccessTokenType:
		fallthrough
	default:
		claims["access_token_uuid"] = uuid
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(param.Secret))
	if err != nil {
		return TokenPair{}, err
	}

	tokenPair.UUID = uuid
	tokenPair.SignedToken = signedToken

	return tokenPair, nil
}

func (t *Tokenizer) GenerateTokenDetail(userID int64) (TokenDetail, error) {
	accessTokenExpiryTime := time.Now().Add(expired1Day)
	refreshTokenExpiryTime := time.Now().Add(expired30Days)
	tokenDetails := TokenDetail{
		AccessTokenExpiredAtTime:  accessTokenExpiryTime,
		AccessTokenExpiredAt:      accessTokenExpiryTime.Unix(),
		RefreshTokenExpiredAtTime: refreshTokenExpiryTime,
		RefreshTokenExpiredAt:     refreshTokenExpiryTime.Unix(),
	}
	var err error
	accessTokenPair, err := generateSingleToken(SingleTokenParam{
		Type:        AccessTokenType,
		UserID:      userID,
		ExpiredTime: tokenDetails.AccessTokenExpiredAt,
		Secret:      t.Config.AccessTokenSecret,
	})
	if err != nil {
		return tokenDetails, err
	}

	tokenDetails.AccessTokenUUID = accessTokenPair.UUID
	tokenDetails.AccessToken = accessTokenPair.SignedToken

	refreshTokenPair, err := generateSingleToken(SingleTokenParam{
		Type:        RefreshTokenType,
		UserID:      userID,
		ExpiredTime: tokenDetails.RefreshTokenExpiredAt,
		Secret:      t.Config.RefreshTokenSecret,
	})
	if err != nil {
		return tokenDetails, err
	}

	tokenDetails.RefreshTokenUUID = refreshTokenPair.UUID
	tokenDetails.RefreshToken = refreshTokenPair.SignedToken

	return tokenDetails, nil
}

func (t *Tokenizer) VerifyToken(jwtToken, tokenType string) (ClaimsDetail, error) {
	claimsDetail := ClaimsDetail{}
	token, err := jwt.Parse(jwtToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}

		switch tokenType {
		case RefreshTokenType:
			return []byte(t.Config.RefreshTokenSecret), nil
		case AccessTokenType:
			fallthrough
		default:
			return []byte(t.Config.AccessTokenSecret), nil
		}
	})
	if err != nil {
		return claimsDetail, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		claimsDetail.Valid = true
		if _, ok := claims["user_id"]; ok {
			claimsDetail.UserID = int64(claims["user_id"].(float64))
		}

		if _, ok := claims["access_token_uuid"]; ok {
			claimsDetail.AccessTokenUUID = claims["access_token_uuid"].(string)
		}

		if _, ok := claims["refresh_token_uuid"]; ok {
			claimsDetail.RefreshTokenUUID = claims["refresh_token_uuid"].(string)
		}
	}

	return claimsDetail, err
}
