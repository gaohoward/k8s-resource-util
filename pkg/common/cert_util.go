package common

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/rsa"
)

func NewRsaKey(len int) (*rsa.PrivateKey, error) {
	caPrivKey, err := rsa.GenerateKey(crand.Reader, len)
	if err != nil {
		return nil, err
	}
	if err := caPrivKey.Validate(); err != nil {
		return nil, err
	}
	return caPrivKey, nil
}

func NewEcdsaKey() (*ecdsa.PrivateKey, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	if err != nil {
		return nil, err
	}
	return priv, nil
}
