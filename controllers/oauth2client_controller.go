// Copyright © 2022 Ory Corp

/*

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

package controllers

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hydrav1alpha1 "github.com/ory/hydra-maester/api/v1alpha1"
	"github.com/ory/hydra-maester/hydra"
)

const (
	ClientIDKey     = "client_id"
	ClientSecretKey = "client_secret"
	FinalizerName   = "finalizer.ory.hydra.sh"

	DefaultNamespace = "default"
)

type clientKey struct {
	url            string
	port           int
	endpoint       string
	forwardedProto string
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
		r.Log.Info((fmt.Sprintf("ControllerNamespace is set to: %s, working only on items in this namespace. Other namespaces are ignored.", r.ControllerNamespace)))
		if req.NamespacedName.Namespace != r.ControllerNamespace {
			r.Log.Info((fmt.Sprintf("Requested resource %s is not in namespace: %s and will be ignored", req.String(), r.ControllerNamespace)))
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
			if registerErr := r.registerOAuth2Client(ctx, &oauth2client, nil); registerErr != nil {
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

	}

	if found {
		//conclude reconciliation if the client exists and has not been updated
		if oauth2client.Generation == oauth2client.Status.ObservedGeneration {
			return ctrl.Result{}, nil
		}

		if fetched.Owner != fmt.Sprintf("%s/%s", oauth2client.Name, oauth2client.Namespace) {
			conflictErr := errors.Errorf("ID provided in secret %s/%s is assigned to another resource", secret.Name, secret.Namespace)
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

	if registerErr := r.registerOAuth2Client(ctx, &oauth2client, credentials); registerErr != nil {
		return ctrl.Result{}, registerErr
	}

	return ctrl.Result{}, nil
}

func (r *OAuth2ClientReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hydrav1alpha1.OAuth2Client{}).
		Complete(r)
}

func (r *OAuth2ClientReconciler) registerOAuth2Client(ctx context.Context, c *hydrav1alpha1.OAuth2Client, credentials *hydra.Oauth2ClientCredentials) error {
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
		return errors.WithStack(err)
	}

	if credentials != nil {
		if _, err := hydraClient.PostOAuth2Client(oauth2client.WithCredentials(credentials)); err != nil {
			if updateErr := r.updateReconciliationStatusError(ctx, c, hydrav1alpha1.StatusRegistrationFailed, err); updateErr != nil {
				return updateErr
			}
		}
		return r.ensureEmptyStatusError(ctx, c)
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
		return errors.WithStack(err)
	}

	if _, err := hydraClient.PutOAuth2Client(oauth2client.WithCredentials(credentials)); err != nil {
		if updateErr := r.updateReconciliationStatusError(ctx, c, hydrav1alpha1.StatusUpdateFailed, err); updateErr != nil {
			return updateErr
		}
	}
	return r.ensureEmptyStatusError(ctx, c)
}

func (r *OAuth2ClientReconciler) unregisterOAuth2Clients(ctx context.Context, c *hydrav1alpha1.OAuth2Client) error {

	// if a reqired field is empty, that means this is a delete after
	// the finalizers have done their job, so just return
	if c.Spec.Scope == "" || c.Spec.SecretName == "" {
		return nil
	}

	hydra, err := r.getHydraClientForClient(*c)
	if err != nil {
		return err
	}

	clients, err := hydra.ListOAuth2Client()
	if err != nil {
		return err
	}

	for _, cJSON := range clients {
		if cJSON.Owner == fmt.Sprintf("%s/%s", c.Name, c.Namespace) {
			if err := hydra.DeleteOAuth2Client(*cJSON.ClientID); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *OAuth2ClientReconciler) updateReconciliationStatusError(ctx context.Context, c *hydrav1alpha1.OAuth2Client, code hydrav1alpha1.StatusCode, err error) error {
	r.Log.Error(err, fmt.Sprintf("error processing client %s/%s ", c.Name, c.Namespace), "oauth2client", "register")
	c.Status.ReconciliationError = hydrav1alpha1.ReconciliationError{
		Code:        code,
		Description: err.Error(),
	}

	return r.updateClientStatus(ctx, c)
}

func (r *OAuth2ClientReconciler) ensureEmptyStatusError(ctx context.Context, c *hydrav1alpha1.OAuth2Client) error {
	c.Status.ReconciliationError = hydrav1alpha1.ReconciliationError{}
	return r.updateClientStatus(ctx, c)
}

func (r *OAuth2ClientReconciler) updateClientStatus(ctx context.Context, c *hydrav1alpha1.OAuth2Client) error {
	c.Status.ObservedGeneration = c.Generation
	if err := r.Status().Update(ctx, c); err != nil {
		r.Log.Error(err, fmt.Sprintf("status update failed for client %s/%s ", c.Name, c.Namespace), "oauth2client", "update status")
		return err
	}
	return nil
}

func parseSecret(secret apiv1.Secret, authMethod hydrav1alpha1.TokenEndpointAuthMethod) (*hydra.Oauth2ClientCredentials, error) {

	id, found := secret.Data[ClientIDKey]
	if !found {
		return nil, errors.New(`"client_id property missing"`)
	}

	psw, found := secret.Data[ClientSecretKey]
	if !found && authMethod != "none" {
		return nil, errors.New(`"client_secret property missing"`)
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
		r.mu.Lock()
		defer r.mu.Unlock()
		if c, ok := r.oauth2Clients[key]; ok {
			return c, nil
		}

		client, err := r.oauth2ClientFactory(spec, "", false)
		if err != nil {
			return nil, errors.Wrap(err, "cannot create oauth2 client from CRD")
		}

		r.oauth2Clients[key] = client
		return client, nil
	}

	if r.HydraClient == nil {
		return nil, errors.New("Not default client or other clients configured")
	}
	r.Log.Info(fmt.Sprintf("using default client"))
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
