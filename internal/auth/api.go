package auth

import (
	"fmt"
	"net/http"
	"strings"
)

func GetAPIKey(headers http.Header) (string, error) {
	authorizationHeader := headers.Get("Authorization")
	apiKey := strings.TrimPrefix(authorizationHeader, "ApiKey ")
	if strings.TrimSpace(apiKey) == "" || apiKey == authorizationHeader {
		return "", fmt.Errorf("api key is missing or invalid")
	}
	return apiKey, nil
}
