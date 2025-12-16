package config

import (
	"os"
	"path/filepath"
)

const AppName = "gogcli"

func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, AppName), nil
}

func EnsureDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// KeyringDir is where the keyring "file" backend stores encrypted entries.
//
// We keep this separate from the main config dir because the file backend creates
// one file per key.
func KeyringDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "keyring"), nil
}

func EnsureKeyringDir() (string, error) {
	dir, err := KeyringDir()
	if err != nil {
		return "", err
	}
	// keyring's file backend uses 0700 by default; match that.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func ClientCredentialsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

func DriveDownloadsDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "drive-downloads"), nil
}

func EnsureDriveDownloadsDir() (string, error) {
	dir, err := DriveDownloadsDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func GmailAttachmentsDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gmail-attachments"), nil
}

func EnsureGmailAttachmentsDir() (string, error) {
	dir, err := GmailAttachmentsDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}
