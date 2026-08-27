//go:build !darwin && !windows && !linux

package platform

import "context"

type unavailableCredentialBackend struct{}

func defaultCredentialBackend() credentialBackend { return unavailableCredentialBackend{} }
func (unavailableCredentialBackend) Presence(context.Context, string, string) (bool, error) {
	return false, errCredentialUnavailable
}
func (unavailableCredentialBackend) Update(context.Context, string, string, []byte) error {
	return errCredentialUnavailable
}
func (unavailableCredentialBackend) Add(context.Context, string, string, []byte) error {
	return errCredentialUnavailable
}
func (unavailableCredentialBackend) Delete(context.Context, string, string) error {
	return errCredentialUnavailable
}
func (unavailableCredentialBackend) Read(context.Context, string, string) ([]byte, error) {
	return nil, errCredentialUnavailable
}
