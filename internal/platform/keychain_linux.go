//go:build linux

package platform

import (
	"context"
	"errors"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	linuxCredentialOperationTimeout = 30 * time.Second
	linuxCredentialCleanupTimeout   = time.Second

	secretServiceName         = "org.freedesktop.secrets"
	secretServicePath         = dbus.ObjectPath("/org/freedesktop/secrets")
	secretServiceInterface    = "org.freedesktop.Secret.Service"
	secretCollectionInterface = "org.freedesktop.Secret.Collection"
	secretItemInterface       = "org.freedesktop.Secret.Item"
	secretSessionInterface    = "org.freedesktop.Secret.Session"
	secretPromptInterface     = "org.freedesktop.Secret.Prompt"
	noSecretServiceObject     = dbus.ObjectPath("/")
)

type linuxCredentialBackend struct{}

type linuxSecretService struct {
	conn   *dbus.Conn
	object dbus.BusObject
}

type linuxSecret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string `dbus:"content_type"`
}

func defaultCredentialBackend() credentialBackend { return linuxCredentialBackend{} }

func (linuxCredentialBackend) Presence(ctx context.Context, service, account string) (bool, error) {
	operationCtx, cancel, err := linuxCredentialOperationContext(ctx)
	if err != nil {
		return false, err
	}
	defer cancel()

	secretService, err := openLinuxSecretService(operationCtx)
	if err != nil {
		return false, translateLinuxCredentialError(operationCtx, err)
	}
	defer secretService.close()

	collection, err := secretService.collection(operationCtx)
	if err != nil {
		return false, translateLinuxCredentialError(operationCtx, err)
	}
	if err := secretService.unlock(operationCtx, collection); err != nil {
		return false, translateLinuxCredentialError(operationCtx, err)
	}
	items, err := secretService.search(operationCtx, collection, service, account)
	if err != nil {
		return false, translateLinuxCredentialError(operationCtx, err)
	}
	return len(items) > 0, nil
}

func (linuxCredentialBackend) Update(ctx context.Context, service, account string, value []byte) error {
	operationCtx, cancel, err := linuxCredentialOperationContext(ctx)
	if err != nil {
		return err
	}
	defer cancel()

	secretService, err := openLinuxSecretService(operationCtx)
	if err != nil {
		return translateLinuxCredentialError(operationCtx, err)
	}
	defer secretService.close()

	collection, err := secretService.collection(operationCtx)
	if err != nil {
		return translateLinuxCredentialError(operationCtx, err)
	}
	if err := secretService.unlock(operationCtx, collection); err != nil {
		return translateLinuxCredentialError(operationCtx, err)
	}
	items, err := secretService.search(operationCtx, collection, service, account)
	if err != nil {
		return translateLinuxCredentialError(operationCtx, err)
	}
	if len(items) == 0 {
		return errCredentialNotFound
	}
	return translateLinuxCredentialError(
		operationCtx,
		secretService.replace(operationCtx, collection, service, account, value),
	)
}

func (linuxCredentialBackend) Add(ctx context.Context, service, account string, value []byte) error {
	operationCtx, cancel, err := linuxCredentialOperationContext(ctx)
	if err != nil {
		return err
	}
	defer cancel()

	secretService, err := openLinuxSecretService(operationCtx)
	if err != nil {
		return translateLinuxCredentialError(operationCtx, err)
	}
	defer secretService.close()

	collection, err := secretService.collection(operationCtx)
	if err != nil {
		return translateLinuxCredentialError(operationCtx, err)
	}
	if err := secretService.unlock(operationCtx, collection); err != nil {
		return translateLinuxCredentialError(operationCtx, err)
	}
	return translateLinuxCredentialError(
		operationCtx,
		secretService.replace(operationCtx, collection, service, account, value),
	)
}

func (linuxCredentialBackend) Delete(ctx context.Context, service, account string) error {
	operationCtx, cancel, err := linuxCredentialOperationContext(ctx)
	if err != nil {
		return err
	}
	defer cancel()

	secretService, err := openLinuxSecretService(operationCtx)
	if err != nil {
		return translateLinuxCredentialError(operationCtx, err)
	}
	defer secretService.close()

	collection, err := secretService.collection(operationCtx)
	if err != nil {
		return translateLinuxCredentialError(operationCtx, err)
	}
	if err := secretService.unlock(operationCtx, collection); err != nil {
		return translateLinuxCredentialError(operationCtx, err)
	}
	items, err := secretService.search(operationCtx, collection, service, account)
	if err != nil {
		return translateLinuxCredentialError(operationCtx, err)
	}
	if len(items) == 0 {
		return errCredentialNotFound
	}
	if err := secretService.unlock(operationCtx, items...); err != nil {
		return translateLinuxCredentialError(operationCtx, err)
	}
	for _, item := range items {
		if err := secretService.delete(operationCtx, item); err != nil {
			return translateLinuxCredentialError(operationCtx, err)
		}
	}
	return nil
}

func (linuxCredentialBackend) Read(ctx context.Context, service, account string) ([]byte, error) {
	operationCtx, cancel, err := linuxCredentialOperationContext(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()

	secretService, err := openLinuxSecretService(operationCtx)
	if err != nil {
		return nil, translateLinuxCredentialError(operationCtx, err)
	}
	defer secretService.close()

	collection, err := secretService.collection(operationCtx)
	if err != nil {
		return nil, translateLinuxCredentialError(operationCtx, err)
	}
	if err := secretService.unlock(operationCtx, collection); err != nil {
		return nil, translateLinuxCredentialError(operationCtx, err)
	}
	items, err := secretService.search(operationCtx, collection, service, account)
	if err != nil {
		return nil, translateLinuxCredentialError(operationCtx, err)
	}
	if len(items) == 0 {
		return nil, errCredentialNotFound
	}
	if len(items) != 1 {
		return nil, errCredentialUnavailable
	}
	if err := secretService.unlock(operationCtx, items[0]); err != nil {
		return nil, translateLinuxCredentialError(operationCtx, err)
	}

	session, err := secretService.openSession(operationCtx)
	if err != nil {
		return nil, translateLinuxCredentialError(operationCtx, err)
	}
	defer secretService.closeSession(operationCtx, session)

	secret, err := secretService.getSecret(operationCtx, items[0], session)
	defer clearLinuxSecret(&secret)
	if err != nil {
		return nil, translateLinuxCredentialError(operationCtx, err)
	}
	if secret.Session != session || len(secret.Value) == 0 {
		return nil, errCredentialNotFound
	}
	return append([]byte(nil), secret.Value...), nil
}

func linuxCredentialOperationContext(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if err := credentialContextError(ctx); err != nil {
		return nil, nil, err
	}
	operationCtx, cancel := context.WithTimeout(ctx, linuxCredentialOperationTimeout)
	return operationCtx, cancel, nil
}

func openLinuxSecretService(ctx context.Context) (*linuxSecretService, error) {
	if err := credentialContextError(ctx); err != nil {
		return nil, err
	}
	conn, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return &linuxSecretService{
		conn:   conn,
		object: conn.Object(secretServiceName, secretServicePath),
	}, nil
}

func (service *linuxSecretService) close() {
	_ = service.conn.Close()
}

func (service *linuxSecretService) collection(ctx context.Context) (dbus.ObjectPath, error) {
	for _, alias := range []string{"default", "login"} {
		var collection dbus.ObjectPath
		err := service.object.CallWithContext(
			ctx,
			secretServiceInterface+".ReadAlias",
			0,
			alias,
		).Store(&collection)
		if err != nil {
			return noSecretServiceObject, err
		}
		if collection != noSecretServiceObject && collection.IsValid() {
			return collection, nil
		}
	}
	return noSecretServiceObject, errCredentialUnavailable
}

func (service *linuxSecretService) openSession(ctx context.Context) (dbus.ObjectPath, error) {
	var output dbus.Variant
	var session dbus.ObjectPath
	err := service.object.CallWithContext(
		ctx,
		secretServiceInterface+".OpenSession",
		0,
		"plain",
		dbus.MakeVariant(""),
	).Store(&output, &session)
	if err != nil {
		return noSecretServiceObject, err
	}
	if session == noSecretServiceObject || !session.IsValid() {
		return noSecretServiceObject, errCredentialUnavailable
	}
	return session, nil
}

func (service *linuxSecretService) closeSession(ctx context.Context, session dbus.ObjectPath) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), linuxCredentialCleanupTimeout)
	defer cancel()
	_ = service.conn.Object(secretServiceName, session).CallWithContext(
		cleanupCtx,
		secretSessionInterface+".Close",
		0,
	).Err
}

func (service *linuxSecretService) search(
	ctx context.Context,
	collection dbus.ObjectPath,
	credentialService string,
	account string,
) ([]dbus.ObjectPath, error) {
	var items []dbus.ObjectPath
	err := service.conn.Object(secretServiceName, collection).CallWithContext(
		ctx,
		secretCollectionInterface+".SearchItems",
		0,
		linuxCredentialAttributes(credentialService, account),
	).Store(&items)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item == noSecretServiceObject || !item.IsValid() {
			return nil, errCredentialUnavailable
		}
	}
	return items, nil
}

func (service *linuxSecretService) unlock(ctx context.Context, objects ...dbus.ObjectPath) error {
	if len(objects) == 0 {
		return nil
	}
	var unlocked []dbus.ObjectPath
	var prompt dbus.ObjectPath
	err := service.object.CallWithContext(
		ctx,
		secretServiceInterface+".Unlock",
		0,
		objects,
	).Store(&unlocked, &prompt)
	if err != nil {
		return err
	}
	_, err = service.prompt(ctx, prompt)
	return err
}

func (service *linuxSecretService) replace(
	ctx context.Context,
	collection dbus.ObjectPath,
	credentialService string,
	account string,
	value []byte,
) error {
	session, err := service.openSession(ctx)
	if err != nil {
		return err
	}
	defer service.closeSession(ctx, session)

	temporary := append([]byte(nil), value...)
	secret := linuxSecret{
		Session:     session,
		Parameters:  []byte{},
		Value:       temporary,
		ContentType: "application/octet-stream",
	}
	defer clearLinuxSecret(&secret)

	properties := map[string]dbus.Variant{
		secretItemInterface + ".Label": dbus.MakeVariant("Fallout Terminal public access: " + account),
		secretItemInterface + ".Attributes": dbus.MakeVariant(
			linuxCredentialAttributes(credentialService, account),
		),
	}
	var item dbus.ObjectPath
	var prompt dbus.ObjectPath
	err = service.conn.Object(secretServiceName, collection).CallWithContext(
		ctx,
		secretCollectionInterface+".CreateItem",
		0,
		properties,
		secret,
		true,
	).Store(&item, &prompt)
	if err != nil {
		return err
	}
	result, err := service.prompt(ctx, prompt)
	if err != nil {
		return err
	}
	if item != noSecretServiceObject && item.IsValid() {
		return nil
	}
	promptItem, ok := result.Value().(dbus.ObjectPath)
	if !ok || promptItem == noSecretServiceObject || !promptItem.IsValid() {
		return errCredentialUnavailable
	}
	return nil
}

func (service *linuxSecretService) getSecret(
	ctx context.Context,
	item dbus.ObjectPath,
	session dbus.ObjectPath,
) (linuxSecret, error) {
	var secret linuxSecret
	err := service.conn.Object(secretServiceName, item).CallWithContext(
		ctx,
		secretItemInterface+".GetSecret",
		0,
		session,
	).Store(&secret)
	return secret, err
}

func (service *linuxSecretService) delete(ctx context.Context, item dbus.ObjectPath) error {
	var prompt dbus.ObjectPath
	err := service.conn.Object(secretServiceName, item).CallWithContext(
		ctx,
		secretItemInterface+".Delete",
		0,
	).Store(&prompt)
	if err != nil {
		return err
	}
	_, err = service.prompt(ctx, prompt)
	return err
}

func (service *linuxSecretService) prompt(ctx context.Context, prompt dbus.ObjectPath) (dbus.Variant, error) {
	if prompt == noSecretServiceObject {
		return dbus.Variant{}, nil
	}
	if !prompt.IsValid() {
		return dbus.Variant{}, errCredentialUnavailable
	}

	options := []dbus.MatchOption{
		dbus.WithMatchObjectPath(prompt),
		dbus.WithMatchInterface(secretPromptInterface),
		dbus.WithMatchMember("Completed"),
	}
	if err := service.conn.AddMatchSignalContext(ctx, options...); err != nil {
		return dbus.Variant{}, err
	}
	signals := make(chan *dbus.Signal, 1)
	service.conn.Signal(signals)
	defer func() {
		service.conn.RemoveSignal(signals)
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), linuxCredentialCleanupTimeout)
		defer cancel()
		_ = service.conn.RemoveMatchSignalContext(cleanupCtx, options...)
	}()

	promptObject := service.conn.Object(secretServiceName, prompt)
	if err := promptObject.CallWithContext(
		ctx,
		secretPromptInterface+".Prompt",
		0,
		"",
	).Err; err != nil {
		return dbus.Variant{}, err
	}

	for {
		select {
		case <-ctx.Done():
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), linuxCredentialCleanupTimeout)
			defer cancel()
			_ = promptObject.CallWithContext(
				cleanupCtx,
				secretPromptInterface+".Dismiss",
				0,
			).Err
			return dbus.Variant{}, ctx.Err()
		case signal, ok := <-signals:
			if !ok || signal == nil {
				return dbus.Variant{}, errCredentialUnavailable
			}
			if signal.Path != prompt || signal.Name != secretPromptInterface+".Completed" {
				continue
			}
			if len(signal.Body) != 2 {
				return dbus.Variant{}, errCredentialUnavailable
			}
			dismissed, ok := signal.Body[0].(bool)
			if !ok {
				return dbus.Variant{}, errCredentialUnavailable
			}
			if dismissed {
				return dbus.Variant{}, errCredentialUserCancelled
			}
			result, ok := signal.Body[1].(dbus.Variant)
			if !ok {
				return dbus.Variant{}, errCredentialUnavailable
			}
			return result, nil
		}
	}
}

func linuxCredentialAttributes(service, account string) map[string]string {
	return map[string]string{
		"application": credentialServiceBase,
		"service":     service,
		"account":     account,
	}
}

func clearLinuxSecret(secret *linuxSecret) {
	if secret == nil {
		return
	}
	clear(secret.Parameters)
	secret.Parameters = nil
	clear(secret.Value)
	secret.Value = nil
}

func translateLinuxCredentialError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, errCredentialNotFound),
		errors.Is(err, errCredentialLocked),
		errors.Is(err, errCredentialDenied),
		errors.Is(err, errCredentialUnavailable),
		errors.Is(err, errCredentialUserCancelled):
		return err
	}

	var dbusErr *dbus.Error
	if !errors.As(err, &dbusErr) {
		return errCredentialUnavailable
	}
	switch dbusErr.Name {
	case "org.freedesktop.Secret.Error.NoSuchObject",
		"org.freedesktop.DBus.Error.UnknownObject":
		return errCredentialNotFound
	case "org.freedesktop.Secret.Error.IsLocked":
		return errCredentialLocked
	case "org.freedesktop.Secret.Error.PermissionDenied",
		"org.freedesktop.DBus.Error.AccessDenied",
		"org.freedesktop.DBus.Error.AuthFailed",
		"org.freedesktop.DBus.Error.InteractiveAuthorizationRequired":
		return errCredentialDenied
	case "org.freedesktop.Secret.Error.Cancelled",
		"org.freedesktop.DBus.Error.Cancelled":
		return errCredentialUserCancelled
	default:
		return errCredentialUnavailable
	}
}

var _ credentialBackend = linuxCredentialBackend{}
