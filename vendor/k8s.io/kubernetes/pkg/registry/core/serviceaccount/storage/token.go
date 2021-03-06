/*
Copyright 2018 The Kubernetes Authors.

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

package storage

import (
	"fmt"

	authenticationapiv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	authenticationapi "k8s.io/kubernetes/pkg/apis/authentication"
	api "k8s.io/kubernetes/pkg/apis/core"
	token "k8s.io/kubernetes/pkg/serviceaccount"
)

func (r *TokenREST) New() runtime.Object {
	return &authenticationapi.TokenRequest{}
}

type TokenREST struct {
	svcaccts getter
	pods     getter
	secrets  getter
	issuer   token.TokenGenerator
}

var _ = rest.NamedCreater(&TokenREST{})

func (r *TokenREST) Create(ctx genericapirequest.Context, name string, obj runtime.Object, createValidation rest.ValidateObjectFunc, includeUninitialized bool) (runtime.Object, error) {
	if err := createValidation(obj); err != nil {
		return nil, err
	}

	out := obj.(*authenticationapi.TokenRequest)

	svcacctObj, err := r.svcaccts.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	svcacct := svcacctObj.(*api.ServiceAccount)

	var (
		pod    *api.Pod
		secret *api.Secret
	)

	if ref := out.Spec.BoundObjectRef; ref != nil {
		var uid types.UID

		gvk := schema.FromAPIVersionAndKind(ref.APIVersion, ref.Kind)
		switch {
		case gvk.Group == "" && gvk.Kind == "Pod":
			podObj, err := r.pods.Get(ctx, ref.Name, &metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			pod = podObj.(*api.Pod)
			uid = pod.UID
		case gvk.Group == "" && gvk.Kind == "Secret":
			secretObj, err := r.secrets.Get(ctx, ref.Name, &metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			secret = secretObj.(*api.Secret)
			uid = secret.UID
		default:
			return nil, errors.NewBadRequest(fmt.Sprintf("cannot bind token to object of type %s", gvk.String()))
		}
		if ref.UID != "" && uid != ref.UID {
			return nil, errors.NewConflict(schema.GroupResource{Group: gvk.Group, Resource: gvk.Kind}, ref.Name, fmt.Errorf("the UID in the bound object reference (%s) does not match the UID in record (%s). The object might have been deleted and then recreated", ref.UID, uid))
		}
	}
	sc, pc := token.Claims(*svcacct, pod, secret, out.Spec.ExpirationSeconds, out.Spec.Audiences)
	tokdata, err := r.issuer.GenerateToken(sc, pc)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %v", err)
	}

	out.Status = authenticationapi.TokenRequestStatus{
		Token:               tokdata,
		ExpirationTimestamp: metav1.Time{Time: sc.Expiry.Time()},
	}
	return out, nil
}

func (r *TokenREST) GroupVersionKind(containingGV schema.GroupVersion) schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   authenticationapiv1.SchemeGroupVersion.Group,
		Version: authenticationapiv1.SchemeGroupVersion.Version,
		Kind:    "TokenRequest",
	}
}

type getter interface {
	Get(ctx genericapirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error)
}
