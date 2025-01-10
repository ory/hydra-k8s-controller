// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package hydra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"

	hydrav1alpha1 "github.com/ory/hydra-maester/api/v1alpha1"
	"github.com/ory/hydra-maester/helpers"
	apiv1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Client interface {
	GetOAuth2Client(id string) (*OAuth2ClientJSON, bool, error)
	ListOAuth2Client() ([]*OAuth2ClientJSON, error)
	PostOAuth2Client(o *OAuth2ClientJSON) (*OAuth2ClientJSON, error)
	PutOAuth2Client(o *OAuth2ClientJSON) (*OAuth2ClientJSON, error)
	DeleteOAuth2Client(id string) error
}

type InternalClient struct {
	HydraURL       url.URL
	HTTPClient     *http.Client
	ForwardedProto string
	ApiKey         string
}

// New returns a new hydra InternalClient instance.
func New(spec hydrav1alpha1.OAuth2ClientSpec, tlsTrustStore string, insecureSkipVerify bool) (Client, error) {
	address := fmt.Sprintf("%s:%d", spec.HydraAdmin.URL, spec.HydraAdmin.Port)
	u, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	c, err := helpers.CreateHttpClient(insecureSkipVerify, tlsTrustStore)
	if err != nil {
		return nil, err
	}

	client := &InternalClient{
		HydraURL:   *u.ResolveReference(&url.URL{Path: spec.HydraAdmin.Endpoint}),
		HTTPClient: c,
	}

	if spec.HydraAdmin.ForwardedProto != "" && spec.HydraAdmin.ForwardedProto != "off" {
		client.ForwardedProto = spec.HydraAdmin.ForwardedProto
	}

	apiKey, err := fetchApiKey(spec.HydraAdmin.ApiKeySecretRef)
	if err != nil {
		return nil, err
	}

	getEnv := os.Getenv("HYDRA_API_KEY")
	if getEnv != "" {
		client.ApiKey = getEnv
	} else if apiKey != "" {
		client.ApiKey = apiKey
	}

	return client, nil
}

func (c *InternalClient) GetOAuth2Client(id string) (*OAuth2ClientJSON, bool, error) {
	var jsonClient *OAuth2ClientJSON

	req, err := c.newRequest(http.MethodGet, id, nil)
	if err != nil {
		return nil, false, err
	}

	resp, err := c.do(req, &jsonClient)
	if err != nil {
		return nil, false, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return jsonClient, true, nil
	case http.StatusNotFound, http.StatusUnauthorized:
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("%s %s http request returned unexpected status code %s", req.Method, req.URL.String(), resp.Status)
	}
}

func (c *InternalClient) ListOAuth2Client() ([]*OAuth2ClientJSON, error) {
	var jsonClientList []*OAuth2ClientJSON

	req, err := c.newRequest(http.MethodGet, "", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(req, &jsonClientList)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return jsonClientList, nil
	default:
		return nil, fmt.Errorf("%s %s http request returned unexpected status code %s", req.Method, req.URL.String(), resp.Status)
	}
}

func (c *InternalClient) PostOAuth2Client(o *OAuth2ClientJSON) (*OAuth2ClientJSON, error) {
	var jsonClient *OAuth2ClientJSON

	req, err := c.newRequest(http.MethodPost, "", o)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(req, &jsonClient)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		return jsonClient, nil
	case http.StatusConflict:
		return nil, fmt.Errorf("%s %s http request failed: requested ID already exists", req.Method, req.URL)
	default:
		return nil, fmt.Errorf("%s %s http request returned unexpected status code: %s", req.Method, req.URL, resp.Status)
	}
}

func (c *InternalClient) PutOAuth2Client(o *OAuth2ClientJSON) (*OAuth2ClientJSON, error) {
	var jsonClient *OAuth2ClientJSON

	req, err := c.newRequest(http.MethodPut, *o.ClientID, o)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(req, &jsonClient)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s %s http request returned unexpected status code: %s", req.Method, req.URL, resp.Status)
	}

	return jsonClient, nil
}

func (c *InternalClient) DeleteOAuth2Client(id string) error {
	req, err := c.newRequest(http.MethodDelete, id, nil)
	if err != nil {
		return err
	}

	resp, err := c.do(req, nil)
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		fmt.Printf("InternalClient with id %s does not exist", id)
		return nil
	default:
		return fmt.Errorf("%s %s http request returned unexpected status code %s", req.Method, req.URL.String(), resp.Status)
	}
}

func (c *InternalClient) newRequest(method, relativePath string, body interface{}) (*http.Request, error) {
	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}

	u := c.HydraURL
	u.Path = path.Join(u.Path, relativePath)

	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}

	if c.ForwardedProto != "" {
		req.Header.Add("X-Forwarded-Proto", c.ForwardedProto)
	}

	if c.ApiKey != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.ApiKey))
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	return req, nil

}

func (c *InternalClient) do(req *http.Request, v interface{}) (*http.Response, error) {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if v != nil && resp.StatusCode < 300 {
		err = json.NewDecoder(resp.Body).Decode(v)
	}
	return resp, err
}

func fetchApiKey(spec hydrav1alpha1.ApiKeySecretRef) (string, error) {

	secretName := spec.Name
	secretNamespace := spec.Namespace

	if secretName == "" || secretNamespace == "" {
		return "", nil
	}

	cfg := ctrl.GetConfigOrDie()
	kubeClient, err := client.New(cfg, client.Options{})
	if err != nil {
		ctrl.Log.Error(err, "unable to create client to fetch secret")
		return "", err
	}

	var secret apiv1.Secret
	err = kubeClient.Get(context.Background(), types.NamespacedName{Name: secretName, Namespace: secretNamespace}, &secret)
	if err != nil {
		if apierrs.IsNotFound(err) {
			ctrl.Log.Error(err, fmt.Sprintf("secret %s/%s does not exist", secretName, secretNamespace))
			return "", fmt.Errorf("secret %s/%s does not exist", secretNamespace, secretName)
		}
		ctrl.Log.Error(err, fmt.Sprintf("error fetching secret %s/%s", secretName, secretNamespace))
		return "", fmt.Errorf("error fetching secret %s/%s: %s", secretNamespace, secretName, err)
	}

	secretValue, ok := secret.Data[spec.Key]
	if !ok {
		ctrl.Log.Error(err, fmt.Sprintf("key %s doesn't exist within secret %s/%s", secretName, secretNamespace, spec.Key))
		return "", fmt.Errorf("key %s doesn't exist within secret %s/%s", secretName, secretNamespace, spec.Key)
	}

	return string(secretValue), nil
}
