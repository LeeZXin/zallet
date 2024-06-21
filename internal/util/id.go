package util

import (
	"github.com/google/uuid"
	"strings"
)

func RandomUuid() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}
