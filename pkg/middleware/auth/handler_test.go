package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const testToken = "test-token"

var testResourceAttrs = ResourceAttributes{
	Group:       "monitoring.coreos.com",
	Version:     "v1",
	Resource:    "prometheuses",
	Namespace:   "d8-monitoring",
	Name:        "main",
	Subresource: "metrics",
}

func newTestMiddleware(t *testing.T, client *fake.Clientset) *Middleware {
	t.Helper()
	return NewMiddlewareFromKubeClient(client, testResourceAttrs)
}

func TestMiddlewareFromKubeClient_UnauthorizedWithoutBearerToken(t *testing.T) {
	t.Parallel()

	client := fake.NewClientset()
	mw := newTestMiddleware(t, client)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "http://example/metrics", nil)
	rr := httptest.NewRecorder()
	mw.Handler(next).ServeHTTP(rr, req)

	require.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMiddlewareFromKubeClient_UnauthorizedWhenTokenReviewNotAuthenticated(t *testing.T) {
	t.Parallel()

	client := fake.NewClientset()
	client.Fake.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &authenticationv1.TokenReview{
			Status: authenticationv1.TokenReviewStatus{
				Authenticated: false,
			},
		}, nil
	})

	mw := newTestMiddleware(t, client)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "http://example/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)

	rr := httptest.NewRecorder()
	mw.Handler(next).ServeHTTP(rr, req)

	require.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMiddlewareFromKubeClient_ForbiddenWhenSARDenied(t *testing.T) {
	t.Parallel()

	client := fake.NewClientset()
	client.Fake.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &authenticationv1.TokenReview{
			Status: authenticationv1.TokenReviewStatus{
				Authenticated: true,
				User: authenticationv1.UserInfo{
					Username: "alice",
					UID:      "u1",
					Groups:   []string{"devs"},
				},
			},
		}, nil
	})
	client.Fake.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &authorizationv1.SubjectAccessReview{
			Status: authorizationv1.SubjectAccessReviewStatus{
				Allowed: false,
				Reason:  "nope",
			},
		}, nil
	})

	mw := newTestMiddleware(t, client)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "http://example/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)

	rr := httptest.NewRecorder()
	mw.Handler(next).ServeHTTP(rr, req)

	require.Equal(t, http.StatusForbidden, rr.Code)
}

func TestMiddlewareFromKubeClient_AllowsAndPassesThrough(t *testing.T) {
	t.Parallel()

	const (
		wantUser = "alice"
		wantVerb = "get"
	)

	client := fake.NewClientset()
	client.Fake.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		ca, ok := action.(k8stesting.CreateAction)
		require.True(t, ok, "expected CreateAction, got %T", action)

		tr, ok := ca.GetObject().(*authenticationv1.TokenReview)
		require.True(t, ok, "expected *TokenReview, got %T", ca.GetObject())
		require.Equal(t, testToken, tr.Spec.Token)

		return true, &authenticationv1.TokenReview{
			ObjectMeta: metav1.ObjectMeta{Name: "ignored"},
			Status: authenticationv1.TokenReviewStatus{
				Authenticated: true,
				User: authenticationv1.UserInfo{
					Username: wantUser,
					UID:      "u1",
					Groups:   []string{"devs"},
				},
			},
		}, nil
	})
	client.Fake.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		ca, ok := action.(k8stesting.CreateAction)
		require.True(t, ok, "expected CreateAction, got %T", action)

		sar, ok := ca.GetObject().(*authorizationv1.SubjectAccessReview)
		require.True(t, ok, "expected *SubjectAccessReview, got %T", ca.GetObject())

		require.Equal(t, wantUser, sar.Spec.User)
		require.NotNil(t, sar.Spec.ResourceAttributes)
		require.Equal(t, wantVerb, sar.Spec.ResourceAttributes.Verb)
		require.Equal(t, testResourceAttrs.Group, sar.Spec.ResourceAttributes.Group)
		require.Equal(t, testResourceAttrs.Version, sar.Spec.ResourceAttributes.Version)
		require.Equal(t, testResourceAttrs.Resource, sar.Spec.ResourceAttributes.Resource)
		require.Equal(t, testResourceAttrs.Subresource, sar.Spec.ResourceAttributes.Subresource)
		require.Equal(t, testResourceAttrs.Namespace, sar.Spec.ResourceAttributes.Namespace)
		require.Equal(t, testResourceAttrs.Name, sar.Spec.ResourceAttributes.Name)

		return true, &authorizationv1.SubjectAccessReview{
			Status: authorizationv1.SubjectAccessReviewStatus{
				Allowed: true,
				Reason:  "ok",
			},
		}, nil
	})

	mw := newTestMiddleware(t, client)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "http://example/metrics", nil)
	req = req.WithContext(context.Background())
	req.Header.Set("Authorization", "Bearer "+testToken)

	rr := httptest.NewRecorder()
	mw.Handler(next).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.True(t, nextCalled)
}
