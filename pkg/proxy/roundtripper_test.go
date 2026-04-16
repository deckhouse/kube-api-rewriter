/*
Copyright 2026 Flant JSC

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

package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestRoundTripper_RewritesCreateRequestAndResponse(t *testing.T) {
	rules := handlerTestRules()
	rules.Init()

	var gotPath string
	var gotBody []byte

	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path

		var err error
		gotBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: io.NopCloser(bytes.NewReader([]byte(`{
				"apiVersion":"internal.virtualization.deckhouse.io/v1",
				"kind":"InternalVirtualizationVirtualMachine",
				"metadata":{"name":"vm1","namespace":"default"}
			}`))),
			Request: r,
		}, nil
	})

	rt := NewProxyRoundTripper("test", ToRenamed, rules)
	rt.Base = base
	client := &http.Client{Transport: rt}

	reqBody := []byte(`{
		"apiVersion":"kubevirt.io/v1",
		"kind":"VirtualMachine",
		"metadata":{"name":"vm1","namespace":"default"}
	}`)
	req, err := http.NewRequest(
		http.MethodPost,
		"https://cluster.example/apis/kubevirt.io/v1/namespaces/default/virtualmachines",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, "/apis/internal.virtualization.deckhouse.io/v1/namespaces/default/internalvirtualizationvirtualmachines", gotPath)
	require.JSONEq(t, `{
		"apiVersion":"internal.virtualization.deckhouse.io/v1",
		"kind":"InternalVirtualizationVirtualMachine",
		"metadata":{"name":"vm1","namespace":"default"}
	}`, string(gotBody))
	require.JSONEq(t, `{
		"apiVersion":"kubevirt.io/v1",
		"kind":"VirtualMachine",
		"metadata":{"name":"vm1","namespace":"default"}
	}`, string(respBody))
}

func TestRoundTripper_RewritesWatchResponse(t *testing.T) {
	rules := handlerTestRules()
	rules.Init()

	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/apis/internal.virtualization.deckhouse.io/v1/namespaces/default/internalvirtualizationvirtualmachines", r.URL.Path)

		ev := metav1.WatchEvent{
			Type: "ADDED",
			Object: runtime.RawExtension{
				Raw: []byte(`{
					"apiVersion":"internal.virtualization.deckhouse.io/v1",
					"kind":"InternalVirtualizationVirtualMachine",
					"metadata":{"name":"vm1","namespace":"default"}
				}`),
			},
		}
		payload, err := json.Marshal(ev)
		require.NoError(t, err)

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body:    io.NopCloser(bytes.NewReader(payload)),
			Request: r,
		}, nil
	})

	rt := NewProxyRoundTripper("test", ToRenamed, rules)
	rt.Base = base
	client := &http.Client{Transport: rt}

	req, err := http.NewRequest(
		http.MethodGet,
		"https://cluster.example/apis/kubevirt.io/v1/namespaces/default/virtualmachines?watch=true",
		nil,
	)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `"kind":"VirtualMachine"`)
	require.Contains(t, string(body), `"apiVersion":"kubevirt.io/v1"`)
	require.NotContains(t, string(body), `InternalVirtualizationVirtualMachine`)
}

func TestRoundTripper_WatchWorksWithRESTClientDecoder(t *testing.T) {
	rules := handlerTestRules()
	rules.Init()

	groupVersion := schema.GroupVersion{Group: "kubevirt.io", Version: "v1"}
	testScheme := runtime.NewScheme()
	testScheme.AddKnownTypeWithName(groupVersion.WithKind("VirtualMachine"), &testVirtualMachine{})
	metav1.AddToGroupVersion(testScheme, schema.GroupVersion{Version: "v1"})
	codecs := serializer.NewCodecFactory(testScheme)

	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/apis/internal.virtualization.deckhouse.io/v1/namespaces/default/internalvirtualizationvirtualmachines", r.URL.Path)

		ev := metav1.WatchEvent{
			Type: "MODIFIED",
			Object: runtime.RawExtension{
				Raw: []byte(`{
					"apiVersion":"internal.virtualization.deckhouse.io/v1",
					"kind":"InternalVirtualizationVirtualMachine",
					"metadata":{"name":"vm1","namespace":"default"}
				}`),
			},
		}
		payload, err := json.Marshal(ev)
		require.NoError(t, err)

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body:    io.NopCloser(bytes.NewReader(payload)),
			Request: r,
		}, nil
	})

	rt := NewProxyRoundTripper("test", ToRenamed, rules)
	rt.Base = base
	httpClient := &http.Client{Transport: rt}

	cfg := &rest.Config{
		Host: "https://cluster.example",
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &groupVersion,
			NegotiatedSerializer: serializer.WithoutConversionCodecFactory{CodecFactory: codecs},
		},
		APIPath: "/apis",
	}

	client, err := rest.RESTClientForConfigAndClient(cfg, httpClient)
	require.NoError(t, err)

	watcher, err := client.Get().
		Namespace("default").
		Resource("virtualmachines").
		VersionedParams(&metav1.ListOptions{Watch: true}, metav1.ParameterCodec).
		Watch(t.Context())
	require.NoError(t, err)
	defer watcher.Stop()

	select {
	case event, ok := <-watcher.ResultChan():
		require.True(t, ok)
		require.Equal(t, "MODIFIED", string(event.Type))
		vm, ok := event.Object.(*testVirtualMachine)
		require.True(t, ok)
		require.Equal(t, "vm1", vm.Name)
		require.Equal(t, "default", vm.Namespace)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for watch event")
	}
}

func TestRoundTripper_WatchWorksWithStreamingSource(t *testing.T) {
	rules := handlerTestRules()
	rules.Init()

	groupVersion := schema.GroupVersion{Group: "kubevirt.io", Version: "v1"}
	testScheme := runtime.NewScheme()
	testScheme.AddKnownTypeWithName(groupVersion.WithKind("VirtualMachine"), &testVirtualMachine{})
	metav1.AddToGroupVersion(testScheme, schema.GroupVersion{Version: "v1"})
	codecs := serializer.NewCodecFactory(testScheme)

	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/apis/internal.virtualization.deckhouse.io/v1/namespaces/default/internalvirtualizationvirtualmachines", r.URL.Path)

		reader, writer := io.Pipe()
		go func() {
			defer writer.Close()
			time.Sleep(50 * time.Millisecond)

			ev := metav1.WatchEvent{
				Type: "MODIFIED",
				Object: runtime.RawExtension{
					Raw: []byte(`{
						"apiVersion":"internal.virtualization.deckhouse.io/v1",
						"kind":"InternalVirtualizationVirtualMachine",
						"metadata":{"name":"vm1","namespace":"default"}
					}`),
				},
			}
			payload, err := json.Marshal(ev)
			require.NoError(t, err)
			_, err = writer.Write(payload)
			require.NoError(t, err)
		}()

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body:    reader,
			Request: r,
		}, nil
	})

	rt := NewProxyRoundTripper("test", ToRenamed, rules)
	rt.Base = base
	httpClient := &http.Client{Transport: rt}

	cfg := &rest.Config{
		Host: "https://cluster.example",
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &groupVersion,
			NegotiatedSerializer: serializer.WithoutConversionCodecFactory{CodecFactory: codecs},
		},
		APIPath: "/apis",
	}

	client, err := rest.RESTClientForConfigAndClient(cfg, httpClient)
	require.NoError(t, err)

	watcher, err := client.Get().
		Namespace("default").
		Resource("virtualmachines").
		VersionedParams(&metav1.ListOptions{Watch: true}, metav1.ParameterCodec).
		Watch(t.Context())
	require.NoError(t, err)
	defer watcher.Stop()

	select {
	case event, ok := <-watcher.ResultChan():
		require.True(t, ok)
		require.Equal(t, "MODIFIED", string(event.Type))
		vm, ok := event.Object.(*testVirtualMachine)
		require.True(t, ok)
		require.Equal(t, "vm1", vm.Name)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for streaming watch event")
	}
}

func TestRoundTripper_WatchDeliversMultipleEvents(t *testing.T) {
	rules := handlerTestRules()
	rules.Init()

	groupVersion := schema.GroupVersion{Group: "kubevirt.io", Version: "v1"}
	testScheme := runtime.NewScheme()
	testScheme.AddKnownTypeWithName(groupVersion.WithKind("VirtualMachine"), &testVirtualMachine{})
	metav1.AddToGroupVersion(testScheme, schema.GroupVersion{Version: "v1"})
	codecs := serializer.NewCodecFactory(testScheme)

	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/apis/internal.virtualization.deckhouse.io/v1/namespaces/default/internalvirtualizationvirtualmachines", r.URL.Path)

		reader, writer := io.Pipe()
		go func() {
			defer writer.Close()

			events := []metav1.WatchEvent{
				{
					Type: "ADDED",
					Object: runtime.RawExtension{
						Raw: []byte(`{
							"apiVersion":"internal.virtualization.deckhouse.io/v1",
							"kind":"InternalVirtualizationVirtualMachine",
							"metadata":{"name":"vm1","namespace":"default","annotations":{"a":"1"}}
						}`),
					},
				},
				{
					Type: "MODIFIED",
					Object: runtime.RawExtension{
						Raw: []byte(`{
							"apiVersion":"internal.virtualization.deckhouse.io/v1",
							"kind":"InternalVirtualizationVirtualMachine",
							"metadata":{"name":"vm1","namespace":"default","annotations":{"a":"2"}}
						}`),
					},
				},
			}

			for _, ev := range events {
				payload, err := json.Marshal(ev)
				require.NoError(t, err)
				_, err = writer.Write(payload)
				require.NoError(t, err)
				time.Sleep(50 * time.Millisecond)
			}
		}()

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body:    reader,
			Request: r,
		}, nil
	})

	rt := NewProxyRoundTripper("test", ToRenamed, rules)
	rt.Base = base
	httpClient := &http.Client{Transport: rt}

	cfg := &rest.Config{
		Host: "https://cluster.example",
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &groupVersion,
			NegotiatedSerializer: serializer.WithoutConversionCodecFactory{CodecFactory: codecs},
		},
		APIPath: "/apis",
	}

	client, err := rest.RESTClientForConfigAndClient(cfg, httpClient)
	require.NoError(t, err)

	watcher, err := client.Get().
		Namespace("default").
		Resource("virtualmachines").
		VersionedParams(&metav1.ListOptions{Watch: true}, metav1.ParameterCodec).
		Watch(t.Context())
	require.NoError(t, err)
	defer watcher.Stop()

	select {
	case event, ok := <-watcher.ResultChan():
		require.True(t, ok)
		require.Equal(t, "ADDED", string(event.Type))
		vm, ok := event.Object.(*testVirtualMachine)
		require.True(t, ok)
		require.Equal(t, "1", vm.Annotations["a"])
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first watch event")
	}

	select {
	case event, ok := <-watcher.ResultChan():
		require.True(t, ok)
		require.Equal(t, "MODIFIED", string(event.Type))
		vm, ok := event.Object.(*testVirtualMachine)
		require.True(t, ok)
		require.Equal(t, "2", vm.Annotations["a"])
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for second watch event")
	}
}

type testVirtualMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

func (t *testVirtualMachine) DeepCopyObject() runtime.Object {
	if t == nil {
		return nil
	}
	copied := *t
	copied.ObjectMeta = *t.ObjectMeta.DeepCopy()
	return &copied
}

func TestRoundTripper_WatchDeliversEventsAfterBookmark(t *testing.T) {
	rules := handlerTestRules()
	rules.Init()

	groupVersion := schema.GroupVersion{Group: "kubevirt.io", Version: "v1"}
	testScheme := runtime.NewScheme()
	testScheme.AddKnownTypeWithName(groupVersion.WithKind("VirtualMachine"), &testVirtualMachine{})
	metav1.AddToGroupVersion(testScheme, schema.GroupVersion{Version: "v1"})
	codecs := serializer.NewCodecFactory(testScheme)

	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		reader, writer := io.Pipe()
		go func() {
			defer writer.Close()

			events := []metav1.WatchEvent{
				{
					Type: "ADDED",
					Object: runtime.RawExtension{
						Raw: []byte(`{
							"apiVersion":"internal.virtualization.deckhouse.io/v1",
							"kind":"InternalVirtualizationVirtualMachine",
							"metadata":{"name":"vm1","namespace":"default","resourceVersion":"100"}
						}`),
					},
				},
				{
					Type: "BOOKMARK",
					Object: runtime.RawExtension{
						Raw: []byte(`{
							"apiVersion":"internal.virtualization.deckhouse.io/v1",
							"kind":"InternalVirtualizationVirtualMachine",
							"metadata":{"resourceVersion":"200"}
						}`),
					},
				},
				{
					Type: "MODIFIED",
					Object: runtime.RawExtension{
						Raw: []byte(`{
							"apiVersion":"internal.virtualization.deckhouse.io/v1",
							"kind":"InternalVirtualizationVirtualMachine",
							"metadata":{"name":"vm1","namespace":"default","resourceVersion":"300","annotations":{"updated":"true"}}
						}`),
					},
				},
			}

			for _, ev := range events {
				payload, err := json.Marshal(ev)
				require.NoError(t, err)
				_, err = writer.Write(payload)
				require.NoError(t, err)
				time.Sleep(50 * time.Millisecond)
			}
		}()

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body:    reader,
			Request: r,
		}, nil
	})

	rt := NewProxyRoundTripper("test", ToRenamed, rules)
	rt.Base = base
	httpClient := &http.Client{Transport: rt}

	cfg := &rest.Config{
		Host: "https://cluster.example",
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &groupVersion,
			NegotiatedSerializer: serializer.WithoutConversionCodecFactory{CodecFactory: codecs},
		},
		APIPath: "/apis",
	}

	client, err := rest.RESTClientForConfigAndClient(cfg, httpClient)
	require.NoError(t, err)

	watcher, err := client.Get().
		Namespace("default").
		Resource("virtualmachines").
		VersionedParams(&metav1.ListOptions{Watch: true}, metav1.ParameterCodec).
		Watch(t.Context())
	require.NoError(t, err)
	defer watcher.Stop()

	select {
	case event, ok := <-watcher.ResultChan():
		require.True(t, ok)
		require.Equal(t, "ADDED", string(event.Type))
		vm, ok := event.Object.(*testVirtualMachine)
		require.True(t, ok)
		require.Equal(t, "vm1", vm.Name)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ADDED event")
	}

	select {
	case event, ok := <-watcher.ResultChan():
		require.True(t, ok)
		require.Equal(t, "BOOKMARK", string(event.Type))
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for BOOKMARK event")
	}

	select {
	case event, ok := <-watcher.ResultChan():
		require.True(t, ok)
		require.Equal(t, "MODIFIED", string(event.Type))
		vm, ok := event.Object.(*testVirtualMachine)
		require.True(t, ok)
		require.Equal(t, "vm1", vm.Name)
		require.Equal(t, "true", vm.Annotations["updated"])
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for MODIFIED event after BOOKMARK")
	}
}
