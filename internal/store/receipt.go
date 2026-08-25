package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/fantasyce/agent-runtime-proof/internal/receipt"
	sdkmodel "github.com/fantasyce/agent-runtime-proof/sdk/model"
)

const maxStoredReceiptBytes = 1 << 20

func WriteReceipt(root string, value sdkmodel.LaunchReceipt) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode launch receipt: %w", err)
	}
	if len(encoded) > maxStoredReceiptBytes {
		return "", errors.New("launch receipt exceeds storage limit")
	}
	if _, err := receipt.Validate(encoded); err != nil {
		return "", fmt.Errorf("validate launch receipt before storage: %w", err)
	}
	digest := strings.TrimPrefix(value.ReceiptID, "sha256:")
	if len(digest) != 64 || digest == value.ReceiptID {
		return "", errors.New("launch receipt has invalid content ID")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve receipt root: %w", err)
	}
	directory := filepath.Join(absRoot, "launch-receipts")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create receipt directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return "", fmt.Errorf("secure receipt directory: %w", err)
	}
	target := filepath.Join(directory, digest+".json")
	temporary, err := os.CreateTemp(directory, ".receipt-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temporary receipt: %w", err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return "", fmt.Errorf("secure temporary receipt: %w", err)
	}
	if _, err := temporary.Write(encoded); err != nil {
		_ = temporary.Close()
		return "", fmt.Errorf("write temporary receipt: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return "", fmt.Errorf("sync temporary receipt: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("close temporary receipt: %w", err)
	}
	if err := os.Link(temporaryPath, target); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("publish launch receipt: %w", err)
		}
		existing, readErr := readBounded(target)
		if readErr != nil {
			return "", fmt.Errorf("read existing launch receipt: %w", readErr)
		}
		if !bytes.Equal(existing, encoded) {
			return "", errors.New("existing launch receipt conflicts with content ID")
		}
	} else if err := syncDirectory(directory); err != nil {
		return "", fmt.Errorf("sync receipt directory: %w", err)
	}
	if err := os.Remove(temporaryPath); err != nil {
		return "", fmt.Errorf("remove temporary receipt: %w", err)
	}
	removeTemporary = false
	return target, nil
}

func readBounded(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, maxStoredReceiptBytes+1))
	if err != nil {
		return nil, err
	}
	if len(contents) > maxStoredReceiptBytes {
		return nil, errors.New("existing launch receipt exceeds storage limit")
	}
	return contents, nil
}
