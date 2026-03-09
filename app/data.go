package app

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
)

func (app *App) CreateData(userID int64, fileName string, data io.Reader) (string, error) {
	filePath := filepath.Join(app.dataDir, strconv.Itoa(int(userID)), fileName)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return "", err
	}

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return "", err
	}
	defer file.Close()
	defer func() {
		if err != nil {
			_ = os.RemoveAll(filePath)
		}
	}()

	_, err = io.Copy(file, data)
	if err != nil {
		return "", err
	}

	return filePath, nil
}
