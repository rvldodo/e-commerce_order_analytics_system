package dto

type VerifyAccessTokenParam struct {
	AccessToken string `json:"access_token" validate:"required"`
}

type VerifyAccessTokenResult struct {
	Valid bool `json:"valid"`
}
