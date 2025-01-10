// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/go-logr/logr"
	apiv1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hydrav1alpha1 "github.com/ory/hydra-maester/api/v1alpha1"
	"github.com/ory/hydra-maester/hydra"
)

const (
	DefaultClientID  = "CLIENT_ID"
	DefaultSecretKey = "CLIENT_SECRET"
	FinalizerName    = "finalizer.ory.hydra.sh"

	DefaultNamespace = "default"
)

var (
	ClientIDKey     = DefaultClientID
	ClientSecretKey = DefaultSecretKey
)

type clientKey struct {
	url             string
	port            int
	endpoint        string
	forwardedProto  string
	apiKeySecretRef ApiKeySecretRef
}

type ApiKeySecretRef struct {
	Name      string
	Namespace string
}

// OAuth2ClientFactory is a function that creates oauth2 client.
// The OAuth2ClientReconciler defaults to use hydra.New and the factory allows
// to override this behavior for mocks during tests.
type OAuth2ClientFactory func(
	spec hydrav1alpha1.OAuth2ClientSpec,
	tlsTrustStore string,
	insecureSkipVerify bool,
) (hydra.Client, error)

// OAuth2ClientReconciler reconciles a OAuth2Client object.
type OAuth2ClientReconciler struct {
	client.Client
	HydraClient         hydra.Client
	Log                 logr.Logger
	ControllerNamespace string

	oauth2Clients       map[clientKey]hydra.Client
	oauth2ClientFactory OAuth2ClientFactory
	mu                  sync.Mutex
}

// Options represent options to pass to the oauth2 client reconciler.
type Options struct {
	Namespace           string
	OAuth2ClientFactory OAuth2ClientFactory
}

// Option is a functional option.
type Option func(*Options)

func init() {
	if os.Getenv("CLIENT_ID_KEY") != "" {
		ClientIDKey = os.Getenv("CLIENT_ID_KEY")
	}
	if os.Getenv("CLIENT_SECRET_KEY") != "" {
		ClientSecretKey = os.Getenv("CLIENT_SECRET_KEY")
	}
}

// WithNamespace sets the kubernetes namespace for the controller.
// The default is "default".
func WithNamespace(ns string) Option {
	return func(o *Options) {
		o.Namespace = ns
	}
}

// WithClientFactory sets a function to create new oauth2 clients during the reconciliation logic.
func WithClientFactory(factory OAuth2ClientFactory) Option {
	return func(o *Options) {
		o.OAuth2ClientFactory = factory
	}
}

// New returns a new Oauth2ClientReconciler.
func New(c client.Client, hydraClient hydra.Client, log logr.Logger, opts ...Option) *OAuth2ClientReconciler {
	options := &Options{
		Namespace:           DefaultNamespace,
		OAuth2ClientFactory: hydra.New,
	}
	for _, opt := range opts {
		opt(options)
	}

	return &OAuth2ClientReconciler{
		Client:              c,
		HydraClient:         hydraClient,
		Log:                 log,
		ControllerNamespace: options.Namespace,
		oauth2Clients:       make(map[clientKey]hydra.Client, 0),
		oauth2ClientFactory: options.OAuth2ClientFactory,
	}
}

// +kubebuilder:rbac:groups=hydra.ory.sh,resources=oauth2clients,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hydra.ory.sh,resources=oauth2clients/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

func (r *OAuth2ClientReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("oauth2client", req.NamespacedName)

	var oauth2client hydrav1alpha1.OAuth2Client
	if err := r.Get(ctx, req.NamespacedName, &oauth2client); err != nil {
		if apierrs.IsNotFound(err) {
			if registerErr := r.unregisterOAuth2Clients(ctx, &oauth2client); registerErr != nil {
				return ctrl.Result{}, registerErr
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Check request namespace
	if r.ControllerNamespace != "" {
		r.Log.Info(fmt.Sprintf("ControllerNamespace is set to: %s, working only on items in this namespace. Other namespaces are ignored.", r.ControllerNamespace))
		if req.NamespacedName.Namespace != r.ControllerNamespace {
			r.Log.Info(fmt.Sprintf("Requested resource %s is not in namespace: %s and will be ignored", req.String(), r.ControllerNamespace))
			return ctrl.Result{}, nil
		}
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if oauth2client.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !containsString(oauth2client.ObjectMeta.Finalizers, FinalizerName) {
			typeMeta := oauth2client.TypeMeta
			oauth2client.ObjectMeta.Finalizers = append(oauth2client.ObjectMeta.Finalizers, FinalizerName)
			if err := r.Update(ctx, &oauth2client); err != nil {
				return ctrl.Result{}, err
			}
			// restore the TypeMeta object as it is removed during Update, but need to be accessed later
			oauth2client.TypeMeta = typeMeta
		}
	} else {
		// The object is being deleted
		if containsString(oauth2client.ObjectMeta.Finalizers, FinalizerName) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.unregisterOAuth2Clients(ctx, &oauth2client); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			oauth2client.ObjectMeta.Finalizers = removeString(oauth2client.ObjectMeta.Finalizers, FinalizerName)
			if err := r.Update(ctx, &oauth2client); err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil

	}

	var secret apiv1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: oauth2client.Spec.SecretName, Namespace: req.Namespace}, &secret); err != nil {
		if apierrs.IsNotFound(err) {
			if registerErr := r.registerOAuth2Client(ctx, &oauth2client); registerErr != nil {
				return ctrl.Result{}, registerErr
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	credentials, err := parseSecret(secret, oauth2client.Spec.TokenEndpointAuthMethod)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("secret %s/%s is invalid", secret.Name, secret.Namespace))
		if updateErr := r.updateReconciliationStatusError(ctx, &oauth2client, hydrav1alpha1.StatusInvalidSecret, err); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}

	hydraClient, err := r.getHydraClientForClient(oauth2client)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf(
			"hydra address %s:%d%s is invalid",
			oauth2client.Spec.HydraAdmin.URL,
			oauth2client.Spec.HydraAdmin.Port,
			oauth2client.Spec.HydraAdmin.Endpoint,
		))
		if updateErr := r.updateReconciliationStatusError(ctx, &oauth2client, hydrav1alpha1.StatusInvalidHydraAddress, err); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}

	fetched, found, err := hydraClient.GetOAuth2Client(string(credentials.ID))
	if err != nil {
		return ctrl.Result{}, err
	} else if !found {
		return ctrl.Result{}, fmt.Errorf("oauth2 client %s not found", credentials.ID)
	}

	if found {
		//conclude reconciliation if the client exists and has not been updated
		if oauth2client.Generation == oauth2client.Status.ObservedGeneration {
			return ctrl.Result{}, nil
		}

		if fetched.Owner != fmt.Sprintf("%s/%s", oauth2client.Name, oauth2client.Namespace) {
			conflictErr := fmt.Errorf("ID provided in secret %s/%s is assigned to another resource", secret.Name, secret.Namespace)
			if updateErr := r.updateReconciliationStatusError(ctx, &oauth2client, hydrav1alpha1.StatusInvalidSecret, conflictErr); updateErr != nil {
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, nil
		}

		if updateErr := r.updateRegisteredOAuth2Client(ctx, &oauth2client, credentials); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *OAuth2ClientReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hydrav1alpha1.OAuth2Client{}).
		Complete(r)
}

func (r *OAuth2ClientReconciler) registerOAuth2Client(ctx context.Context, c *hydrav1alpha1.OAuth2Client) error {
	if err := r.unregisterOAuth2Clients(ctx, c); err != nil {
		return err
	}

	hydraClient, err := r.getHydraClientForClient(*c)
	if err != nil {
		return err
	}

	oauth2client, err := hydra.FromOAuth2Client(c)
	if err != nil {
		if updateErr := r.updateReconciliationStatusError(ctx, c, hydrav1alpha1.StatusRegistrationFailed, err); updateErr != nil {
			return updateErr
		}

		return fmt.Errorf("failed to construct hydra client for object: %w", err)
	}

	created, err := hydraClient.PostOAuth2Client(oauth2client)
	if err != nil {
		if updateErr := r.updateReconciliationStatusError(ctx, c, hydrav1alpha1.StatusRegistrationFailed, err); updateErr != nil {
			return updateErr
		}
		return nil
	}

	clientSecret := apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Spec.SecretName,
			Namespace: c.Namespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: c.TypeMeta.APIVersion,
				Kind:       c.TypeMeta.Kind,
				Name:       c.ObjectMeta.Name,
				UID:        c.ObjectMeta.UID,
			}},
		},
		Data: map[string][]byte{
			ClientIDKey: []byte(*created.ClientID),
		},
	}

	if created.Secret != nil {
		clientSecret.Data[ClientSecretKey] = []byte(*created.Secret)
	}

	if err := r.Create(ctx, &clientSecret); err != nil {
		if updateErr := r.updateReconciliationStatusError(ctx, c, hydrav1alpha1.StatusCreateSecretFailed, err); updateErr != nil {
			return updateErr
		}
	}

	return r.ensureEmptyStatusError(ctx, c)
}

func (r *OAuth2ClientReconciler) updateRegisteredOAuth2Client(ctx context.Context, c *hydrav1alpha1.OAuth2Client, credentials *hydra.Oauth2ClientCredentials) error {
	hydraClient, err := r.getHydraClientForClient(*c)
	if err != nil {
		return err
	}

	oauth2client, err := hydra.FromOAuth2Client(c)
	if err != nil {
		if updateErr := r.updateReconciliationStatusError(ctx, c, hydrav1alpha1.StatusUpdateFailed, err); updateErr != nil {
			return updateErr
		}

		return fmt.Errorf("failed to construct hydra client for object: %w", err)
	}

	if _, err := hydraClient.PutOAuth2Client(oauth2client.WithCredentials(credentials)); err != nil {
		if updateErr := r.updateReconciliationStatusError(ctx, c, hydrav1alpha1.StatusUpdateFailed, err); updateErr != nil {
			return updateErr
		}
	}
	return r.ensureEmptyStatusError(ctx, c)
}

func (r *OAuth2ClientReconciler) unregisterOAuth2Clients(ctx context.Context, c *hydrav1alpha1.OAuth2Client) error {
	// if a required field is empty, that means this is deleted after
	// the finalizers have done their job, so just return
	if c.Spec.Scope == "" || c.Spec.SecretName == "" {
		return nil
	}

	h, err := r.getHydraClientForClient(*c)
	if err != nil {
		return err
	}

	clients, err := h.ListOAuth2Client()
	if err != nil {
		return err
	}

	for _, cJSON := range clients {
		if cJSON.Owner == fmt.Sprintf("%s/%s", c.Name, c.Namespace) {
			if c.Spec.DeletionPolicy == hydrav1alpha1.OAuth2ClientDeletionPolicyOrphan {
				// Do not delete the OAuth2 client.
				r.Log.Info("oauth2 client deletion, leave the row orphan")
				return nil
			}
			if err := h.DeleteOAuth2Client(*cJSON.ClientID); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *OAuth2ClientReconciler) updateReconciliationStatusError(ctx context.Context, c *hydrav1alpha1.OAuth2Client, code hydrav1alpha1.StatusCode, err error) error {
	r.Log.Error(err, fmt.Sprintf("error processing client %s/%s ", c.Name, c.Namespace), "oauth2client", "register")

	_, err = controllerutil.CreateOrPatch(ctx, r.Client, c, func() error {
		c.Status.ObservedGeneration = c.Generation
		c.Status.ReconciliationError = hydrav1alpha1.ReconciliationError{
			Code:        code,
			Description: err.Error(),
		}
		c.Status.Conditions = []hydrav1alpha1.OAuth2ClientCondition{
			{
				Type:   hydrav1alpha1.OAuth2ClientConditionReady,
				Status: hydrav1alpha1.ConditionFalse,
			},
		}

		return nil
	})
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("status update failed for client %s/%s ", c.Name, c.Namespace), "oauth2client", "update status")
	}

	return err
}

func (r *OAuth2ClientReconciler) ensureEmptyStatusError(ctx context.Context, c *hydrav1alpha1.OAuth2Client) error {
	_, err := controllerutil.CreateOrPatch(ctx, r.Client, c, func() error {
		c.Status.ObservedGeneration = c.Generation
		c.Status.ReconciliationError = hydrav1alpha1.ReconciliationError{}
		c.Status.Conditions = []hydrav1alpha1.OAuth2ClientCondition{
			{
				Type:   hydrav1alpha1.OAuth2ClientConditionReady,
				Status: hydrav1alpha1.ConditionTrue,
			},
		}

		return nil
	})
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("status update failed for client %s/%s ", c.Name, c.Namespace), "oauth2client", "update status")
	}

	return err
}

func parseSecret(secret apiv1.Secret, authMethod hydrav1alpha1.TokenEndpointAuthMethod) (*hydra.Oauth2ClientCredentials, error) {
	id, found := secret.Data[ClientIDKey]
	if !found {
		return nil, fmt.Errorf("%s property missing", ClientIDKey)
	}

	psw, found := secret.Data[ClientSecretKey]
	if !found && authMethod != "none" {
		return nil, fmt.Errorf("%s property missing", ClientSecretKey)
	}

	return &hydra.Oauth2ClientCredentials{
		ID:       id,
		Password: psw,
	}, nil
}

func (r *OAuth2ClientReconciler) getHydraClientForClient(
	oauth2client hydrav1alpha1.OAuth2Client) (hydra.Client, error) {
	spec := oauth2client.Spec
	if spec.HydraAdmin.URL != "" {
		key := clientKey{
			url:            spec.HydraAdmin.URL,
			port:           spec.HydraAdmin.Port,
			endpoint:       spec.HydraAdmin.Endpoint,
			forwardedProto: spec.HydraAdmin.ForwardedProto,
		}

		secretName := determineApiSecretName(&spec.HydraAdmin)
		secretNamespace := determineApiSecretNamespace(&oauth2client)

		if secretName != "" && secretNamespace != "" {
			key.apiKeySecretRef = ApiKeySecretRef{
				Name:      secretName,
				Namespace: secretNamespace,
			}
			spec.HydraAdmin.ApiKeySecretRef.Name = secretName
			spec.HydraAdmin.ApiKeySecretRef.Namespace = secretNamespace
			if spec.HydraAdmin.ApiKeySecretRef.Key == "" {
				spec.HydraAdmin.ApiKeySecretRef.Key = "hydra_api_key"
			}
		}

		r.mu.Lock()
		defer r.mu.Unlock()
		if c, ok := r.oauth2Clients[key]; ok {
			return c, nil
		}

		c, err := r.oauth2ClientFactory(spec, "", false)
		if err != nil {
			return nil, fmt.Errorf("cannot create oauth2 c from CRD: %w", err)
		}

		r.oauth2Clients[key] = c
		return c, nil
	}

	if r.HydraClient == nil {
		return nil, fmt.Errorf("no default client configured")
	}

	r.Log.Info("Using default client")

	return r.HydraClient, nil

}

// Helper functions to check and remove string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func determineApiSecretName(spec *hydrav1alpha1.HydraAdmin) string {
	if spec.ApiKeySecretRef.Name != "" {
		return spec.ApiKeySecretRef.Name
	}
	return ""
}

func determineApiSecretNamespace(spec *hydrav1alpha1.OAuth2Client) string {
	if spec.Spec.HydraAdmin.ApiKeySecretRef.Namespace != "" {
		return spec.Spec.HydraAdmin.ApiKeySecretRef.Namespace
	}
	return spec.ObjectMeta.Namespace
}
