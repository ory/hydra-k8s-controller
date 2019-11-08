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

	"github.com/go-logr/logr"
	hydrav1alpha2 "github.com/ory/hydra-maester/api/v1alpha2"
	"github.com/ory/hydra-maester/hydra"
	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ClientIDKey     = "client_id"
	ClientSecretKey = "client_secret"
)

type HydraClientInterface interface {
	GetOAuth2Client(id string) (*hydra.OAuth2ClientJSON, bool, error)
	ListOAuth2Client() ([]*hydra.OAuth2ClientJSON, error)
	PostOAuth2Client(o *hydra.OAuth2ClientJSON) (*hydra.OAuth2ClientJSON, error)
	PutOAuth2Client(o *hydra.OAuth2ClientJSON) (*hydra.OAuth2ClientJSON, error)
	DeleteOAuth2Client(id string) error
}

// OAuth2ClientReconciler reconciles a OAuth2Client object
type OAuth2ClientReconciler struct {
	HydraClient HydraClientInterface
	Log         logr.Logger
	client.Client
}

// +kubebuilder:rbac:groups=hydra.ory.sh,resources=oauth2clients,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hydra.ory.sh,resources=oauth2clients/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

func (r *OAuth2ClientReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("oauth2client", req.NamespacedName)

	var oauth2client hydrav1alpha2.OAuth2Client
	if err := r.Get(ctx, req.NamespacedName, &oauth2client); err != nil {
		if apierrs.IsNotFound(err) {
			if registerErr := r.unregisterOAuth2Clients(ctx, req.Name, req.Namespace); registerErr != nil {
				return ctrl.Result{}, registerErr
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if oauth2client.Generation != oauth2client.Status.ObservedGeneration {

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

		credentials, err := parseSecret(secret)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("secret %s/%s is invalid", secret.Name, secret.Namespace))
			if updateErr := r.updateReconciliationStatusError(ctx, &oauth2client, hydrav1alpha2.StatusInvalidSecret, err); updateErr != nil {
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, nil
		}

		fetched, found, err := r.HydraClient.GetOAuth2Client(string(credentials.ID))
		if err != nil {
			return ctrl.Result{}, err

		}

		if found {
			if fetched.Owner != oauth2client.Name {
				conflictErr := errors.Errorf("ID provided in secret %s/%s is assigned to another resource", secret.Name, secret.Namespace)
				if updateErr := r.updateReconciliationStatusError(ctx, &oauth2client, hydrav1alpha2.StatusInvalidSecret, conflictErr); updateErr != nil {
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
	}

	return ctrl.Result{}, nil
}

func (r *OAuth2ClientReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hydrav1alpha2.OAuth2Client{}).
		Complete(r)
}

func (r *OAuth2ClientReconciler) registerOAuth2Client(ctx context.Context, c *hydrav1alpha2.OAuth2Client, credentials *hydra.Oauth2ClientCredentials) error {
	if err := r.unregisterOAuth2Clients(ctx, c.Name, c.Namespace); err != nil {
		return err
	}

	if credentials != nil {
		if _, err := r.HydraClient.PostOAuth2Client(c.ToOAuth2ClientJSON().WithCredentials(credentials)); err != nil {
			if updateErr := r.updateReconciliationStatusError(ctx, c, hydrav1alpha2.StatusRegistrationFailed, err); updateErr != nil {
				return updateErr
			}
		}
		return r.ensureEmptyStatusError(ctx, c)
	}

	created, err := r.HydraClient.PostOAuth2Client(c.ToOAuth2ClientJSON())
	if err != nil {
		if updateErr := r.updateReconciliationStatusError(ctx, c, hydrav1alpha2.StatusRegistrationFailed, err); updateErr != nil {
			return updateErr
		}
		return nil
	}

	clientSecret := apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Spec.SecretName,
			Namespace: c.Namespace,
		},
		Data: map[string][]byte{
			ClientIDKey:     []byte(*created.ClientID),
			ClientSecretKey: []byte(*created.Secret),
		},
	}

	if err := r.Create(ctx, &clientSecret); err != nil {
		if updateErr := r.updateReconciliationStatusError(ctx, c, hydrav1alpha2.StatusCreateSecretFailed, err); updateErr != nil {
			return updateErr
		}
	}

	return r.ensureEmptyStatusError(ctx, c)
}

func (r *OAuth2ClientReconciler) updateRegisteredOAuth2Client(ctx context.Context, c *hydrav1alpha2.OAuth2Client, credentials *hydra.Oauth2ClientCredentials) error {
	if _, err := r.HydraClient.PutOAuth2Client(c.ToOAuth2ClientJSON().WithCredentials(credentials)); err != nil {
		if updateErr := r.updateReconciliationStatusError(ctx, c, hydrav1alpha2.StatusUpdateFailed, err); updateErr != nil {
			return updateErr
		}
	}
	return r.ensureEmptyStatusError(ctx, c)
}

func (r *OAuth2ClientReconciler) unregisterOAuth2Clients(ctx context.Context, name, namespace string) error {

	clients, err := r.HydraClient.ListOAuth2Client()
	if err != nil {
		return err
	}

	for _, c := range clients {
		if c.Owner == fmt.Sprintf("%s/%s", name, namespace) {
			if err := r.HydraClient.DeleteOAuth2Client(*c.ClientID); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *OAuth2ClientReconciler) updateReconciliationStatusError(ctx context.Context, c *hydrav1alpha2.OAuth2Client, code hydrav1alpha2.StatusCode, err error) error {
	r.Log.Error(err, fmt.Sprintf("error processing client %s/%s ", c.Name, c.Namespace), "oauth2client", "register")
	c.Status.ReconciliationError = hydrav1alpha2.ReconciliationError{
		Code:        code,
		Description: err.Error(),
	}

	return r.updateClientStatus(ctx, c)
}

func (r *OAuth2ClientReconciler) ensureEmptyStatusError(ctx context.Context, c *hydrav1alpha2.OAuth2Client) error {
	c.Status.ReconciliationError = hydrav1alpha2.ReconciliationError{}
	return r.updateClientStatus(ctx, c)
}

func (r *OAuth2ClientReconciler) updateClientStatus(ctx context.Context, c *hydrav1alpha2.OAuth2Client) error {
	c.Status.ObservedGeneration = c.Generation
	if err := r.Status().Update(ctx, c); err != nil {
		r.Log.Error(err, fmt.Sprintf("status update failed for client %s/%s ", c.Name, c.Namespace), "oauth2client", "update status")
		return err
	}
	return nil
}

func parseSecret(secret apiv1.Secret) (*hydra.Oauth2ClientCredentials, error) {

	id, found := secret.Data[ClientIDKey]
	if !found {
		return nil, errors.New(`"client_id property missing"`)
	}

	psw, found := secret.Data[ClientSecretKey]
	if !found {
		return nil, errors.New(`"client_secret property missing"`)
	}

	return &hydra.Oauth2ClientCredentials{
		ID:       id,
		Password: psw,
	}, nil
}
