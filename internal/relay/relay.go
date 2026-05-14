package relay

import (
	"errors"
	"strings"
)

const (
	// PodstackDefault is the relay used when no flag, env, or override is set.
	// Set to croc's public relay; operators running their own relay should
	// override at runtime via --relay or PODSTACK_RELAY.
	PodstackDefault = "croc.schollz.com:9009"
	// CrocPublicDefault is the same as PodstackDefault. It exists so the
	// --relay-default flag keeps a stable identity even though it now
	// resolves to the same address as the built-in default.
	CrocPublicDefault = "croc.schollz.com:9009"
	// defaultPort is the canonical croc relay port.
	defaultPort = "9009"
)

// ErrConflictingFlags is returned when both --relay and --relay-default are set.
var ErrConflictingFlags = errors.New("--relay and --relay-default are mutually exclusive")

// Resolve picks the relay address using the documented precedence:
// flagRelay (--relay <x>) > flagDefault (--relay-default) > env (PODSTACK_RELAY) > PodstackDefault.
//
// If the chosen value has no ":port" it is appended with the default croc port.
func Resolve(flagRelay string, flagDefault bool, env string) (string, error) {
	if flagRelay != "" && flagDefault {
		return "", ErrConflictingFlags
	}
	switch {
	case flagRelay != "":
		return ensurePort(flagRelay), nil
	case flagDefault:
		return CrocPublicDefault, nil
	case env != "":
		return ensurePort(env), nil
	default:
		return PodstackDefault, nil
	}
}

func ensurePort(host string) string {
	if strings.Contains(host, ":") {
		return host
	}
	return host + ":" + defaultPort
}
