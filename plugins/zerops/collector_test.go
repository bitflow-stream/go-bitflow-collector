package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/bitflow-stream/go-bitflow/bitflow"
	"github.com/bitflow-stream/go-bitflow/script/reg"
	zeropsv1 "github.com/citlab/zerops-operator/pkg/apis/zerops/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	source1   = "zsource-1"
	source2   = "zsource-2"
	namespace = "default"
)

func TestCollector(t *testing.T) {
	fmt.Println("Running TestCollector")
	registry := reg.NewProcessorRegistry(bitflow.NewEndpointFactory())
	RegisterZeropsDataSourceNotifier("zerops-notify", registry)
	//t.Errorf("Error: Test not implemented")
}

func TestZeropsDataSourceNotifier(t *testing.T) {
	fmt.Println("Running TestZeropsDataSourceNotifier")
	notifier := makeNotifier()
	res := notifier.Expired("test", make([]string, 0))
	if !res {
		t.Errorf("Unexpected result: %v", res)
	}
	assertSources(t, 2, notifier)
	res = notifier.Expired(source1, make([]string, 0))
	if !res {
		t.Errorf("Unexpected result: %v", res)
	}
	assertSources(t, 1, notifier)
}

func TestNotifierUpdate(t *testing.T) {
	fmt.Println("Running TestNotifierUpdate")
	notifier := makeNotifier()
	empty := make([]string, 0)
	notifier.Updated("newSource", makeBitflowSample(), empty)
	assertSources(t, 3, notifier)
	notifier.Updated("newSource", makeBitflowSample(), empty)
	assertSources(t, 3, notifier)
}

func TestReadRequest(t *testing.T) {
	fmt.Println("Running TestReadRequest")
	readFunc := readRequest(200)
	response := "Hello world"
	resp := &http.Response{
		Body:       ioutil.NopCloser(bytes.NewReader([]byte(response))),
		StatusCode: 200,
	}
	str, err := readFunc(resp, nil)
	if str != response {
		t.Errorf("Weird response, expected %s, but was %s", response, str)
	}
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	resp = &http.Response{
		Body:       ioutil.NopCloser(bytes.NewReader([]byte(response))),
		StatusCode: 200,
	}
	readFunc = readRequest(500)
	str, err = readFunc(resp, nil)
	if str != response {
		t.Errorf("Weird response, expected %s, but was %s", response, str)
	}
	if err == nil {
		t.Errorf("Error expected")
	}
}
func makeNotifier() *ZeropsDataSourceNotifier {
	url, _ := url.Parse("http://:9090")
	pod := &corev1.Pod{}
	mapi := map[string]string{"test": "ja", "comp": "collector"}
	datasources := map[string]string{source1: source1, source2: source2}
	return &ZeropsDataSourceNotifier{
		Orchestrator:      "127.0.0.1",
		DataSourceName:    "zerops-ds",
		DataSourceLabels:  mapi,
		DataSourceBaseUrl: url,
		dataSourceNames:   datasources,
		client:            makeFakeClient(),
		pod:               pod,
		namespace:         "default",
	}
}

func makeFakeClient() client.Client {
	var objects []runtime.Object
	objects = append(objects, GetZsource(source1, namespace, nil))
	objects = append(objects, GetZsource(source2, namespace, nil))
	zstep := &zeropsv1.ZerOpsStep{}
	zsource := &zeropsv1.ZerOpsDataSource{}
	stepList := &zeropsv1.ZerOpsStepList{}
	sourceList := &zeropsv1.ZerOpsDataSourceList{}

	s := scheme.Scheme
	s.AddKnownTypes(zeropsv1.SchemeGroupVersion, zstep, zsource, stepList, sourceList)
	cl := fake.NewFakeClientWithScheme(s, objects...)
	return cl
}

func makeBitflowSample() *bitflow.Sample {
	values := make([]bitflow.Value, 2)
	values[0] = 1.0
	values[0] = 2.0
	sample := &bitflow.Sample{
		Values: values,
		Time:   time.Now(),
	}
	sample.SetTag("tag1", "test")
	sample.SetTag("tag2", "dev")
	return sample
}

func GetZsource(name, namespace string, labels map[string]string) *zeropsv1.ZerOpsDataSource {
	return &zeropsv1.ZerOpsDataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: zeropsv1.ZerOpsDataSourceSpec{
			URL: "http://example.com",
		},
	}
}

func assertSources(t *testing.T, count int, notifier *ZeropsDataSourceNotifier) {
	sourceList := &zeropsv1.ZerOpsDataSourceList{}
	err := notifier.client.List(context.TODO(), &client.ListOptions{}, sourceList)
	if err != nil {
		t.Error("Unexpected Error:", err)
	} else {
		counter := 0
		for _, _ = range sourceList.Items {
			counter++
		}
		if counter != count {
			t.Errorf("Expected to find sources; Expected %d, but found %d", count, counter)
		}
	}
}
