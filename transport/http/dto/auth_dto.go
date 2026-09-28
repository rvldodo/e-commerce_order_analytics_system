package dto

import "time"

type LoginParam struct {
	Email string `json:"email" validate:"required,email,max=255" example:"john.doe@example.com"`
}

type LoginResult struct {
	Customer             CustomerResult       `json:"customer"`
	AuthenticationTokens AuthenticationTokens `json:"authentication_tokens"`
}

type CustomerResult struct {
	ID      int64  `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Country string `json:"country"`
}

type AuthenticationTokens struct {
	AccessToken            string    `json:"access_token"`
	RefreshToken           string    `json:"refresh_token"`
	AccessTokenExpiryTime  time.Time `json:"access_token_expiry_time"`
	RefreshTokenExpiryTime time.Time `json:"refresh_token_expiry_time"`
}
