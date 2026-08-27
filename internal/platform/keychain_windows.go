//go:build windows

package platform

import (
	"context"
	"errors"

	"github.com/danieljoos/wincred"
	"golang.org/x/sys/windows"
)

const windowsCredentialComment = "Fallout Terminal public access"

type windowsCredentialBackend struct{}

func defaultCredentialBackend() credentialBackend { return windowsCredentialBackend{} }

func (windowsCredentialBackend) Presence(ctx context.Context, service, account string) (bool, error) {
	if err := credentialContextError(ctx); err != nil {
		return false, err
	}
	target := windowsCredentialTarget(service, account)
	credentials, err := wincred.FilteredList(target + "*")
	if err != nil {
		return false, translateWindowsCredentialError(err)
	}
	present := false
	for _, credential := range credentials {
		if credential == nil {
			continue
		}
		present = present || credential.TargetName == target
		clearWindowsCredential(credential)
	}
	return present, nil
}

func (windowsCredentialBackend) Update(ctx context.Context, service, account string, value []byte) error {
	if err := credentialContextError(ctx); err != nil {
		return err
	}
	credential, err := wincred.GetGenericCredential(windowsCredentialTarget(service, account))
	if err != nil {
		return translateWindowsCredentialError(err)
	}
	if credential == nil {
		return errCredentialUnavailable
	}
	setWindowsCredential(credential, account, value)
	defer clearWindowsCredential(&credential.Credential)
	return translateWindowsCredentialError(credential.Write())
}

func (windowsCredentialBackend) Add(ctx context.Context, service, account string, value []byte) error {
	if err := credentialContextError(ctx); err != nil {
		return err
	}
	credential := wincred.NewGenericCredential(windowsCredentialTarget(service, account))
	setWindowsCredential(credential, account, value)
	defer clearWindowsCredential(&credential.Credential)
	return translateWindowsCredentialError(credential.Write())
}

func (windowsCredentialBackend) Delete(ctx context.Context, service, account string) error {
	if err := credentialContextError(ctx); err != nil {
		return err
	}
	credential := wincred.NewGenericCredential(windowsCredentialTarget(service, account))
	return translateWindowsCredentialError(credential.Delete())
}

func (windowsCredentialBackend) Read(ctx context.Context, service, account string) ([]byte, error) {
	if err := credentialContextError(ctx); err != nil {
		return nil, err
	}
	credential, err := wincred.GetGenericCredential(windowsCredentialTarget(service, account))
	if err != nil {
		return nil, translateWindowsCredentialError(err)
	}
	if credential == nil {
		return nil, errCredentialUnavailable
	}
	defer clearWindowsCredential(&credential.Credential)
	if len(credential.CredentialBlob) == 0 {
		return nil, errCredentialNotFound
	}
	return append([]byte(nil), credential.CredentialBlob...), nil
}

func windowsCredentialTarget(service, account string) string {
	return service + "/" + account
}

func setWindowsCredential(credential *wincred.GenericCredential, account string, value []byte) {
	clear(credential.CredentialBlob)
	credential.CredentialBlob = append([]byte(nil), value...)
	credential.UserName = account
	credential.Comment = windowsCredentialComment
	credential.Persist = wincred.PersistLocalMachine
}

func clearWindowsCredential(credential *wincred.Credential) {
	if credential == nil {
		return
	}
	clear(credential.CredentialBlob)
	credential.CredentialBlob = nil
	for index := range credential.Attributes {
		clear(credential.Attributes[index].Value)
		credential.Attributes[index].Value = nil
	}
}

func translateWindowsCredentialError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, wincred.ErrElementNotFound), errors.Is(err, windows.ERROR_FILE_NOT_FOUND):
		return errCredentialNotFound
	case errors.Is(err, windows.ERROR_NOT_LOGGED_ON), errors.Is(err, windows.ERROR_NO_SUCH_LOGON_SESSION):
		return errCredentialLocked
	case errors.Is(err, windows.ERROR_ACCESS_DENIED),
		errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD),
		errors.Is(err, windows.ERROR_LOGON_FAILURE),
		errors.Is(err, windows.ERROR_ACCOUNT_RESTRICTION):
		return errCredentialDenied
	case errors.Is(err, windows.ERROR_CANCELLED), errors.Is(err, windows.ERROR_OPERATION_ABORTED):
		return errCredentialUserCancelled
	default:
		return errCredentialUnavailable
	}
}

var _ credentialBackend = windowsCredentialBackend{}
