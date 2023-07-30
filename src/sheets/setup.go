package sheets

import (
	"context"
	"encoding/base64"
	"fmt"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"os"
)

var service *sheets.Service

func setup(ctx context.Context) (*sheets.Service, *drive.Service, error) {
	// get bytes from base64 encoded google service accounts key
	credBytes, err := base64.StdEncoding.DecodeString(os.Getenv("KEY_JSON_BASE64"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to base64 decode KEY_JSON_BASE64: %w", err)
	}

	// authenticate and get configuration
	config, err := google.JWTConfigFromJSON(credBytes, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get config from json: %w", err)
	}

	// create client with config and context
	client := config.Client(ctx)

	// create new service using client
	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, nil, err
	}

	// create a new context and set up the Drive service
	driveService, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, nil, fmt.Errorf("unable to retrieve Drive client: %v", err)
	}

	return srv, driveService, nil
}

func Init(ctx context.Context) error {
	var err error
	service, _, err = setup(ctx)
	return err
}
