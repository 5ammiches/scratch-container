package identity

import (
	"crypto/rand"
	"encoding/hex"
	math_rand "math/rand/v2"
	"strings"
)

func GenerateName() string {
	adj := adjectives[math_rand.IntN(len(adjectives))]
	name := names[math_rand.IntN(len(names))]

	return strings.Join([]string{
		adj,
		name,
	}, "_")

	// return fmt.Sprintf("%s-%s", adj, name)
}

func GenerateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
