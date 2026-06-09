package plugin

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

func nowMicros() int64 {
	return time.Now().UTC().UnixMicro()
}

func genID() string {
	return uuid.NewString()
}

func jsonString(v map[string]any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
