package gphotosfb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/adrg/xdg"
	"golang.org/x/oauth2"
)

// getClient retrieves a token, saves the token, then returns the generated client.
func getClient(ctx context.Context, config *oauth2.Config) (*http.Client, error) {
	tokFile, err := xdg.CacheFile("gphotos-fb/token.json")
	if err != nil {
		return nil, fmt.Errorf("xdg.CacheFile: %w", err)
	}

	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok, err := getTokenFromWeb(ctx, config)
		if err != nil {
			return nil, fmt.Errorf("getTokenFromWeb: %w", err)
		}

		if err := saveToken(tokFile, tok); err != nil {
			return nil, fmt.Errorf("saveToken: %w", err)
		}
	}
	return config.Client(ctx, tok), nil
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("fmt.Scan: %w", err)
	}

	tok, err := config.Exchange(ctx, authCode)
	if err != nil {
		return nil, fmt.Errorf("config.Exchange: %w", err)
	}
	return tok, nil
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("os.Open: %w", err)
	}
	defer f.Close()

	tok := &oauth2.Token{}
	if err := json.NewDecoder(f).Decode(tok); err != nil {
		return nil, fmt.Errorf("json.NewDecoder(f).Decode: %w", err)
	}
	return tok, nil
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("os.OpenFile: %w", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}
