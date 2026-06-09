package util

import (
	"fmt"
	"math/rand"
	"time"
)

func GetNowAndLenRandomString(length int) string {
	now := time.Now().UnixMilli()
	randomPart := rand.Intn(1e9)
	s := fmt.Sprintf("%d%09d", now, randomPart)
	if len(s) > length {
		s = s[:length]
	}
	return s
}
