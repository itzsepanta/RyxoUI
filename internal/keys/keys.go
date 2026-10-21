// Package keys makes the key material that the protocols of the panel need.
//
// The panel never invents a key on its own: an operator asks for one, looks
// at it and puts it where it belongs. The keys are shown once and are not
// stored by the panel.
package keys

import (
	"crypto/ecdh"
	"crypto/mlkem"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
)

// X25519 is a key pair of the curve X25519.
//
// Reality and the keys of the clients use this curve.
type X25519 struct {
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"privateKey"`
}

// GenerateX25519 makes a key pair of the curve X25519.
func GenerateX25519() (X25519, error) {
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return X25519{}, fmt.Errorf("keys: generate the key: %w", err)
	}
	return X25519{
		PublicKey:  base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		PrivateKey: base64.RawURLEncoding.EncodeToString(key.Bytes()),
	}, nil
}

// ECH is the key of a domain fronting.
//
// The private key stays on the server that fronts the domain, the public key
// and the configuration list go to the client.
type ECH struct {
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"privateKey"`
	Config     string `json:"config"`
}

// The numbers of the format of a configuration list.
const (
	echVersion         = 0xfe
	echVersionLow      = 0x0d
	echKEMDHKEMP256    = 0x0020
	echDefaultPublicNm = "cloudflare.com"
)

// GenerateECH makes the key of a domain fronting and its configuration list.
//
// The public name is the name the client sees, so it is the name of the
// server that hides the real one.
func GenerateECH(publicName string) (ECH, error) {
	publicName = strings.TrimSpace(publicName)
	if publicName == "" {
		publicName = echDefaultPublicNm
	}
	if len(publicName) > 255 {
		return ECH{}, fmt.Errorf("keys: the public name is too long")
	}

	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return ECH{}, fmt.Errorf("keys: generate the key: %w", err)
	}

	configID, err := randomByte()
	if err != nil {
		return ECH{}, err
	}

	public := key.PublicKey().Bytes()

	// The contents of the configuration:
	//   the id of the configuration, the suite of the key exchange, the public
	//   key, the length of the name, the name itself and the extensions.
	contents := make([]byte, 0, 1+2+2+len(public)+1+len(publicName)+2)
	contents = append(contents, configID)
	contents = binary.BigEndian.AppendUint16(contents, echKEMDHKEMP256)
	contents = binary.BigEndian.AppendUint16(contents, uint16(len(public)))
	contents = append(contents, public...)
	contents = append(contents, byte(len(publicName)))
	contents = append(contents, publicName...)
	contents = binary.BigEndian.AppendUint16(contents, 0)

	// The list of the configurations: the version, the length and the
	// contents.
	list := make([]byte, 0, 3+len(contents))
	list = append(list, echVersion, echVersionLow)
	list = binary.BigEndian.AppendUint16(list, uint16(len(contents)))
	list = append(list, contents...)

	return ECH{
		PublicKey:  base64.StdEncoding.EncodeToString(public),
		PrivateKey: base64.StdEncoding.EncodeToString(key.Bytes()),
		Config:     base64.StdEncoding.EncodeToString(list),
	}, nil
}

// VLESSEncryption is the key of the encryption of the vless protocol.
//
// The client key is the public one and goes to the client, the server key
// stays on the server.
type VLESSEncryption struct {
	ClientKey string `json:"clientKey"`
	ServerKey string `json:"serverKey"`
}

// GenerateVLESSEncryption makes the key of the encryption of the vless
// protocol.
//
// The key of the client is the public one of the key exchange, the key of
// the server is the seed of the private one.
func GenerateVLESSEncryption() (VLESSEncryption, error) {
	key, err := mlkem.GenerateKey768()
	if err != nil {
		return VLESSEncryption{}, fmt.Errorf("keys: generate the key: %w", err)
	}
	return VLESSEncryption{
		ClientKey: base64.StdEncoding.EncodeToString(key.EncapsulationKey().Bytes()),
		ServerKey: base64.StdEncoding.EncodeToString(key.Bytes()),
	}, nil
}

// randomByte reads one random byte that is not zero.
func randomByte() (byte, error) {
	buf := make([]byte, 1)
	for {
		if _, err := rand.Read(buf); err != nil {
			return 0, fmt.Errorf("keys: read the random bytes: %w", err)
		}
		if buf[0] != 0 {
			return buf[0], nil
		}
	}
}
