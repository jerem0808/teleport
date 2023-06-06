/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tbot

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	libdefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tlsca"
)

const renewalRetryLimit = 5

func (b *Bot) renewDestinationsLoop(
	ctx context.Context, reloadChan <-chan struct{},
) error {
	b.log.Infof(
		"Beginning destination renewal loop: ttl=%s interval=%s",
		b.cfg.CertificateTTL,
		b.cfg.RenewalInterval,
	)

	ticker := time.NewTicker(b.cfg.RenewalInterval)
	jitter := retryutils.NewJitter()
	defer ticker.Stop()
	for {
		var err error
		for attempt := 1; attempt <= renewalRetryLimit; attempt++ {
			b.log.Infof(
				"Renewing destinations. Attempt %d of %d.",
				attempt,
				renewalRetryLimit,
			)
			err = b.renewDestinations(ctx)
			if err == nil {
				break
			}

			if attempt != renewalRetryLimit {
				// exponentially back off with jitter, starting at 1 second.
				backoffTime := time.Second * time.Duration(math.Pow(2, float64(attempt-1)))
				backoffTime = jitter(backoffTime)
				b.log.WithError(err).Warnf(
					"Destination renewal attempt %d of %d failed. Retrying after %s.",
					attempt,
					renewalRetryLimit,
					backoffTime,
				)
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(backoffTime):
				}
			}
		}
		if err != nil {
			b.log.Warnf("%d retry attempts exhausted renewing destinations. Waiting for next normal renewal cycle in %s.", renewalRetryLimit, b.cfg.RenewalInterval)
		} else {
			b.log.Infof("Renewed destinations. Next destination renewal in approximately %s.", b.cfg.RenewalInterval)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			continue
		case <-reloadChan:
			continue
		}
	}
}

// generateKeys generates TLS and SSH keypairs.
func generateKeys() (private, sshpub, tlspub []byte, err error) {
	privateKey, publicKey, err := native.GenerateKeyPair()
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	sshPrivateKey, err := ssh.ParseRawPrivateKey(privateKey)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	tlsPublicKey, err := tlsca.MarshalPublicKeyFromPrivateKeyPEM(sshPrivateKey)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	return privateKey, publicKey, tlsPublicKey, nil
}

// describeTLSIdentity generates an informational message about the given
// TLS identity, appropriate for user-facing log messages.
func describeTLSIdentity(log logrus.FieldLogger, ident *identity.Identity) string {
	failedToDescribe := "failed-to-describe"
	cert := ident.X509Cert
	if cert == nil {
		log.Warn("Attempted to describe TLS identity without TLS credentials.")
		return failedToDescribe
	}

	tlsIdent, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		log.WithError(err).Warn("Bot TLS certificate can not be parsed as an identity")
		return failedToDescribe
	}

	var principals []string
	for _, principal := range tlsIdent.Principals {
		if !strings.HasPrefix(principal, constants.NoLoginPrefix) {
			principals = append(principals, principal)
		}
	}

	duration := cert.NotAfter.Sub(cert.NotBefore)
	return fmt.Sprintf(
		"valid: after=%v, before=%v, duration=%s | kind=tls, renewable=%v, disallow-reissue=%v, roles=%v, principals=%v, generation=%v",
		cert.NotBefore.Format(time.RFC3339),
		cert.NotAfter.Format(time.RFC3339),
		duration,
		tlsIdent.Renewable,
		tlsIdent.DisallowReissue,
		tlsIdent.Groups,
		principals,
		tlsIdent.Generation,
	)
}

// identityConfigurator is a function that alters a cert request
type identityConfigurator = func(req *proto.UserCertsRequest)

// generateIdentity uses an identity to retrieve an impersonated identity.
// The `configurator` function, if not nil, can be used to add additional
// requests to the certificate request, for example to add `RouteToDatabase`
// and similar fields, however in that case it must be called with an
// impersonated identity that already has the relevant permissions, much like
// `tsh (app|db|kube) login` is already used to generate an additional set of
// certs.
func (b *Bot) generateIdentity(
	ctx context.Context,
	client auth.ClientI,
	currentIdentity *identity.Identity,
	destCfg *config.DestinationConfig,
	defaultRoles []string,
	configurator identityConfigurator,
) (*identity.Identity, error) {
	// TODO: enforce expiration > renewal period (by what margin?)
	//   This should be ignored if a renewal has been triggered manually or
	//   by a CA rotation.

	// Generate a fresh keypair for the impersonated identity. We don't care to
	// reuse keys here: impersonated certs might not be as well-protected so
	// constantly rotating private keys
	privateKey, publicKey, err := native.GenerateKeyPair()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var roleRequests []string
	if len(destCfg.Roles) > 0 {
		roleRequests = destCfg.Roles
	} else {
		b.log.Debugf("Destination specified no roles, defaults will be requested: %v", defaultRoles)
		roleRequests = defaultRoles
	}

	req := proto.UserCertsRequest{
		PublicKey:      publicKey,
		Username:       currentIdentity.X509Cert.Subject.CommonName,
		Expires:        time.Now().Add(b.cfg.CertificateTTL),
		RoleRequests:   roleRequests,
		RouteToCluster: currentIdentity.ClusterName,

		// Make sure to specify this is an impersonated cert request. If unset,
		// auth cannot differentiate renewable vs impersonated requests when
		// len(roleRequests) == 0.
		UseRoleRequests: true,
	}

	if configurator != nil {
		configurator(&req)
	}

	// First, ask the auth server to generate a new set of certs with a new
	// expiration date.
	certs, err := client.GenerateUserCerts(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The root CA included with the returned user certs will only contain the
	// Teleport User CA. We'll also need the host CA for future API calls.
	localCA, err := client.GetClusterCACert(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caCerts, err := tlsca.ParseCertificatePEMs(localCA.TLSCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Append the host CAs from the auth server.
	for _, cert := range caCerts {
		pemBytes, err := tlsca.MarshalCertificatePEM(cert)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certs.TLSCACerts = append(certs.TLSCACerts, pemBytes)
	}

	// Do not trust SSH CA certs as returned by GenerateUserCerts() with an
	// impersonated identity. It only returns the SSH UserCA in this context,
	// but we also need the HostCA and can't directly set `includeHostCA` as
	// part of the UserCertsRequest.
	// Instead, copy the SSHCACerts from the primary identity.
	certs.SSHCACerts = currentIdentity.SSHCACertBytes

	newIdentity, err := identity.ReadIdentityFromStore(&identity.LoadIdentityParams{
		PrivateKeyBytes: privateKey,
		PublicKeyBytes:  publicKey,
	}, certs, identity.DestinationKinds()...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newIdentity, nil
}

func getDatabase(ctx context.Context, client auth.ClientI, name string) (types.Database, error) {
	res, err := client.ListResources(ctx, proto.ListResourcesRequest{
		Namespace:           defaults.Namespace,
		ResourceType:        types.KindDatabaseServer,
		PredicateExpression: fmt.Sprintf(`name == "%s"`, name),
		Limit:               int32(defaults.DefaultChunkSize),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := types.ResourcesWithLabels(res.Resources).AsDatabaseServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var databases []types.Database
	for _, server := range servers {
		databases = append(databases, server.GetDatabase())
	}

	databases = types.DeduplicateDatabases(databases)
	if len(databases) == 0 {
		return nil, trace.NotFound("database %q not found", name)
	}

	return databases[0], nil
}

func (b *Bot) getRouteToDatabase(ctx context.Context, client auth.ClientI, dbCfg *config.Database) (proto.RouteToDatabase, error) {
	if dbCfg.Service == "" {
		return proto.RouteToDatabase{}, nil
	}

	db, err := getDatabase(ctx, client, dbCfg.Service)
	if err != nil {
		return proto.RouteToDatabase{}, trace.Wrap(err)
	}

	username := dbCfg.Username
	if db.GetProtocol() == libdefaults.ProtocolMongoDB && username == "" {
		// This isn't strictly a runtime error so killing the process seems
		// wrong. We'll just loudly warn about it.
		b.log.Errorf("Database `username` field for %q is unset but is required for MongoDB databases.", dbCfg.Service)
	} else if db.GetProtocol() == libdefaults.ProtocolRedis && username == "" {
		// Per tsh's lead, fall back to the default username.
		username = libdefaults.DefaultRedisUsername
	}

	return proto.RouteToDatabase{
		ServiceName: dbCfg.Service,
		Protocol:    db.GetProtocol(),
		Database:    dbCfg.Database,
		Username:    username,
	}, nil
}

func getApp(ctx context.Context, client auth.ClientI, appName string) (types.Application, error) {
	res, err := client.ListResources(ctx, proto.ListResourcesRequest{
		Namespace:           defaults.Namespace,
		ResourceType:        types.KindAppServer,
		PredicateExpression: fmt.Sprintf(`name == "%s"`, appName),
		Limit:               1,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := types.ResourcesWithLabels(res.Resources).AsAppServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var apps []types.Application
	for _, server := range servers {
		apps = append(apps, server.GetApp())
	}
	apps = types.DeduplicateApps(apps)

	if len(apps) == 0 {
		return nil, trace.BadParameter("app %q not found", appName)
	}

	return apps[0], nil
}

func (b *Bot) getRouteToApp(ctx context.Context, botIdentity *identity.Identity, client auth.ClientI, appCfg *config.App) (proto.RouteToApp, error) {
	if appCfg.App == "" {
		return proto.RouteToApp{}, trace.BadParameter("App name must be configured")
	}

	app, err := getApp(ctx, client, appCfg.App)
	if err != nil {
		return proto.RouteToApp{}, trace.Wrap(err)
	}

	// TODO: AWS?
	ws, err := client.CreateAppSession(ctx, types.CreateAppSessionRequest{
		ClusterName: botIdentity.ClusterName,
		Username:    botIdentity.X509Cert.Subject.CommonName,
		PublicAddr:  app.GetPublicAddr(),
	})
	if err != nil {
		return proto.RouteToApp{}, trace.Wrap(err)
	}

	err = auth.WaitForAppSession(ctx, ws.GetName(), ws.GetUser(), client)
	if err != nil {
		return proto.RouteToApp{}, trace.Wrap(err)
	}

	return proto.RouteToApp{
		Name:        app.GetName(),
		SessionID:   ws.GetName(),
		PublicAddr:  app.GetPublicAddr(),
		ClusterName: botIdentity.ClusterName,
	}, nil
}

// generateImpersonatedIdentity generates an impersonated identity for a given
// destination. It also returns a client that is authenticated with that
// impersonated identity.
func (b *Bot) generateImpersonatedIdentity(
	ctx context.Context,
	botClient auth.ClientI,
	botIdentity *identity.Identity,
	destCfg *config.DestinationConfig,
	defaultRoles []string,
) (impersonatedIdentity *identity.Identity, impersonatedClient auth.ClientI, err error) {
	impersonatedIdentity, err = b.generateIdentity(
		ctx, botClient, botIdentity, destCfg, defaultRoles, nil,
	)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// create a client that uses the impersonated identity, so that when we
	// fetch information, we can ensure access rights are enforced.
	impersonatedClient, err = b.AuthenticatedUserClientFromIdentity(ctx, impersonatedIdentity)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer func() {
		// In success cases, this client is used by the caller and they manage
		// closing, in failure cases, we need to close the client.
		if err != nil && impersonatedClient != nil {
			impersonatedClient.Close()
		}
	}()

	// Now that we have an initial impersonated identity, we can use it to
	// request any app/db/etc certs
	if destCfg.Database != nil {
		route, err := b.getRouteToDatabase(ctx, impersonatedClient, destCfg.Database)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		// The impersonated identity is not allowed to reissue certificates,
		// so we'll request the database access identity using the main bot
		// identity (having gathered the necessary info for RouteToDatabase
		// using the correct impersonated unroutedIdentity.)
		routedIdentity, err := b.generateIdentity(ctx, botClient, impersonatedIdentity, destCfg, defaultRoles, func(req *proto.UserCertsRequest) {
			req.RouteToDatabase = route
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		b.log.Infof("Generated identity for database %q", destCfg.Database.Service)

		return routedIdentity, impersonatedClient, nil
	} else if destCfg.KubernetesCluster != nil {
		// Note: the Teleport server does attempt to verify k8s cluster names
		// and will fail to generate certs if the cluster doesn't exist or is
		// offline.
		routedIdentity, err := b.generateIdentity(ctx, botClient, impersonatedIdentity, destCfg, defaultRoles, func(req *proto.UserCertsRequest) {
			req.KubernetesCluster = destCfg.KubernetesCluster.ClusterName
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		b.log.Infof("Generated identity for Kubernetes cluster %q", *destCfg.KubernetesCluster)

		return routedIdentity, impersonatedClient, nil
	} else if destCfg.App != nil {
		routeToApp, err := b.getRouteToApp(ctx, botIdentity, impersonatedClient, destCfg.App)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		routedIdentity, err := b.generateIdentity(ctx, botClient, impersonatedIdentity, destCfg, defaultRoles, func(req *proto.UserCertsRequest) {
			req.RouteToApp = routeToApp
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		b.log.Infof("Generated identity for app %q", *destCfg.App)

		return routedIdentity, impersonatedClient, nil
	} else if destCfg.Cluster != "" {
		routedIdentity, err := b.generateIdentity(ctx, botClient, impersonatedIdentity, destCfg, defaultRoles, func(req *proto.UserCertsRequest) {
			req.RouteToCluster = destCfg.Cluster
		},
		)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return routedIdentity, impersonatedClient, nil
	}

	return impersonatedIdentity, impersonatedClient, nil
}

// fetchDefaultRoles requests the bot's own role from the auth server and
// extracts its full list of allowed roles.
func fetchDefaultRoles(ctx context.Context, roleGetter services.RoleGetter, botRole string) ([]string, error) {
	role, err := roleGetter.GetRole(ctx, botRole)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conditions := role.GetImpersonateConditions(types.Allow)
	return conditions.Roles, nil
}

// renewDestinations performs a single renewal
func (b *Bot) renewDestinations(
	ctx context.Context,
) error {
	botIdentity := b.ident()
	client, err := b.AuthenticatedUserClientFromIdentity(ctx, botIdentity)
	if err != nil {
		return trace.Wrap(err)
	}
	defer client.Close()

	// create a cache shared across destinations so they don't hammer the auth
	// server with similar requests
	drc := &destinationRenewalCache{
		client: client,
	}

	// Determine the default role list based on the bot role. The role's
	// name should match the certificate's Key ID (user and role names
	// should all match bot-$name)
	botResourceName := botIdentity.X509Cert.Subject.CommonName
	defaultRoles, err := fetchDefaultRoles(ctx, client, botResourceName)
	if err != nil {
		b.log.WithError(err).Warnf("Unable to determine default roles, no roles will be requested if unspecified")
		defaultRoles = []string{}
	}

	// Next, generate impersonated certs
	for _, dest := range b.cfg.Destinations {
		destImpl, err := dest.GetDestination()
		if err != nil {
			return trace.Wrap(err)
		}

		// Check the ACLs. We can't fix them, but we can warn if they're
		// misconfigured. We'll need to precompute a list of keys to check.
		// Note: This may only log a warning, depending on configuration.
		if err := destImpl.Verify(identity.ListKeys(identity.DestinationKinds()...)); err != nil {
			return trace.Wrap(err)
		}

		// Ensure this destination is also writable. This is a hard fail if
		// ACLs are misconfigured, regardless of configuration.
		// TODO: consider not making these a hard error? e.g. write other
		// destinations even if this one is broken?
		if err := identity.VerifyWrite(destImpl); err != nil {
			return trace.Wrap(err, "Could not write to destination %s, aborting.", destImpl)
		}

		impersonatedIdentity, impersonatedClient, err := b.generateImpersonatedIdentity(
			ctx, client, botIdentity, dest, defaultRoles,
		)
		if err != nil {
			return trace.Wrap(err, "Failed to generate impersonated certs for %s: %+v", destImpl, err)
		}
		defer impersonatedClient.Close()

		b.log.Infof("Renewed destination certificates for %s, %s", destImpl, describeTLSIdentity(b.log, impersonatedIdentity))

		if err := identity.SaveIdentity(impersonatedIdentity, destImpl, identity.DestinationKinds()...); err != nil {
			return trace.Wrap(err, "failed to save impersonated identity to destination %s", destImpl)
		}

		// Create a destination provider to bundle up all the dependencies that
		// a destination template might need to render.
		dp := &destinationProvider{
			destinationRenewalCache: drc,
			impersonatedClient:      impersonatedClient,
		}

		for _, templateConfig := range dest.Configs {
			template, err := templateConfig.GetConfigTemplate()
			if err != nil {
				return trace.Wrap(err)
			}

			if err := template.Render(ctx, dp, impersonatedIdentity, dest); err != nil {
				b.log.WithError(err).Warnf("Failed to render config template %+v", templateConfig)
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// destinationRenewalCache is used to cache information during a renewal to pass
// to destinations. This prevents them all hammering the auth server with
// requests for the same information. This is shared between all of the
// destinations.
type destinationRenewalCache struct {
	client auth.ClientI

	mu sync.Mutex
	// These are protected by getter/setters with mutex locks
	_cas       map[types.CertAuthType][]types.CertAuthority
	_authPong  *proto.PingResponse
	_proxyPong *webclient.PingResponse
}

// certAuthorities returns cached CAs of the given type.
func (drc *destinationRenewalCache) certAuthorities(
	caType types.CertAuthType,
) []types.CertAuthority {
	drc.mu.Lock()
	defer drc.mu.Unlock()

	return drc._cas[caType]
}

// GetCertAuthorities returns the possibly cached CAs of the given type and
// requests them from the server if unavailable.
func (drc *destinationRenewalCache) GetCertAuthorities(
	ctx context.Context, caType types.CertAuthType,
) ([]types.CertAuthority, error) {
	if cas := drc.certAuthorities(caType); len(cas) > 0 {
		return cas, nil
	}

	cas, err := drc.client.GetCertAuthorities(ctx, caType, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	drc.mu.Lock()
	defer drc.mu.Unlock()

	if drc._cas == nil {
		drc._cas = map[types.CertAuthType][]types.CertAuthority{}
	}
	drc._cas[caType] = cas
	return cas, nil
}

// AuthPing pings the auth server and returns the (possibly cached) response.
func (drc *destinationRenewalCache) AuthPing(ctx context.Context) (*proto.PingResponse, error) {
	drc.mu.Lock()
	defer drc.mu.Unlock()
	if drc._authPong != nil {
		return drc._authPong, nil
	}

	pong, err := drc.client.Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	drc._authPong = &pong

	return &pong, nil
}

// ProxyPing returns a (possibly cached) ping response from the Teleport proxy.
// Note that it relies on the auth server being configured with a sane proxy
// public address.
func (drc *destinationRenewalCache) ProxyPing(ctx context.Context) (*webclient.PingResponse, error) {
	drc.mu.Lock()
	defer drc.mu.Unlock()
	if drc._proxyPong != nil {
		return drc._proxyPong, nil
	}

	// Note: this relies on the auth server's proxy address. We could
	// potentially support some manual parameter here in the future if desired.
	authPong, err := drc.AuthPing(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyPong, err := webclient.Ping(&webclient.Config{
		Context:   ctx,
		ProxyAddr: authPong.ProxyPublicAddr,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	drc._proxyPong = proxyPong

	return proxyPong, nil
}

// destinationProvider bundles the dependencies a template needs in order to
// produce its output.
// It provides a handy point for controlling what templates are allowed to call
// and how they are allowed to call them. This makes ensuring that they call
// RPCs with the correct identity much easier.
type destinationProvider struct {
	// we embed the cache shared across all destinations to provide access to
	// non-identity specific methods like `AuthPing`.
	*destinationRenewalCache
	// impersonatedClient is a client using the impersonated identity configured
	// for that destination.
	impersonatedClient auth.ClientI
	cfg                *config.BotConfig
}

// GetRemoteClusters uses the impersonatedClient to call GetRemoteClusters.
func (dp *destinationProvider) GetRemoteClusters(opts ...services.MarshalOption) ([]types.RemoteCluster, error) {
	return dp.impersonatedClient.GetRemoteClusters(opts...)
}

// GenerateHostCert uses the impersonatedClient to call GenerateHostCert.
func (dp *destinationProvider) GenerateHostCert(ctx context.Context, key []byte, hostID, nodeName string, principals []string, clusterName string, role types.SystemRole, ttl time.Duration) ([]byte, error) {
	return dp.impersonatedClient.GenerateHostCert(ctx, key, hostID, nodeName, principals, clusterName, role, ttl)
}

// GetCertAuthority uses the impersonatedClient to call GetCertAuthority.
func (dp *destinationProvider) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
	return dp.impersonatedClient.GetCertAuthority(ctx, id, loadKeys)
}

// Config returns the bots config.
func (dp *destinationProvider) Config() *config.BotConfig {
	return dp.cfg
}