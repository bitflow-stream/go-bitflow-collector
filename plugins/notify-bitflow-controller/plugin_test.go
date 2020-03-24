package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"

	bitflowv1 "github.com/bitflow-stream/bitflow-k8s-operator/bitflow-controller/pkg/apis/bitflow/v1"
	"github.com/bitflow-stream/bitflow-k8s-operator/bitflow-controller/pkg/common"
	"github.com/bitflow-stream/go-bitflow/bitflow"
	"github.com/bitflow-stream/go-bitflow/script/reg"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ControllerNotifierTestSuite struct {
	common.AbstractTestSuite
}

func TestControllerNotifier(t *testing.T) {
	new(ControllerNotifierTestSuite).Run(t)
}

func (s *ControllerNotifierTestSuite) TestCollector() {
	registry := reg.NewProcessorRegistry(bitflow.NewEndpointFactory())
	RegisterBitflowDataSourceNotifier("notify-bitflow-controller", registry)
}

func (s *ControllerNotifierTestSuite) TestBitflowSourceNotifier() {
	notifier := s.makeNotifier()
	s.True(notifier.Expired("test", make([]string, 0)))
	s.assertSources(notifier.client, 2)
	s.True(notifier.Expired("source1", make([]string, 0)))
	s.assertSources(notifier.client, 2)
}

func (s *ControllerNotifierTestSuite) TestNotifierUpdate() {
	notifier := s.makeNotifier()
	empty := make([]string, 0)
	notifier.Updated("newSource", s.makeBitflowSample(), empty)
	s.assertSources(notifier.client, 3)
	notifier.Updated("newSource", s.makeBitflowSample(), empty)
	s.assertSources(notifier.client, 3)
}

func (s *ControllerNotifierTestSuite) TestReadRequest() {
	readFunc := readRequest(200)
	response := "Hello world"
	resp := &http.Response{
		Body:       ioutil.NopCloser(bytes.NewReader([]byte(response))),
		StatusCode: 200,
	}
	str, err := readFunc(resp, nil)
	s.NoError(err)
	s.Equal(response, str, "Wrong response")

	resp = &http.Response{
		Body:       ioutil.NopCloser(bytes.NewReader([]byte(response))),
		StatusCode: 200,
	}
	readFunc = readRequest(500)
	str, err = readFunc(resp, nil)
	s.NoError(err)
	s.Equal(response, str)
}

func (s *ControllerNotifierTestSuite) makeNotifier() *BitflowControllerNotifier {
	cl := s.MakeFakeClient(
		s.Source("source2", nil),
		s.Source("source1", nil))
	parsedUrl, _ := url.Parse("http://:9090")
	pod := &corev1.Pod{}
	labels := map[string]string{"test": "ja", "comp": "collector"}
	sources := map[string]string{"source1": "source1", "source2": "source2"}
	return &BitflowControllerNotifier{
		Orchestrator:      "127.0.0.1",
		DataSourceName:    "bitflow-source-1",
		DataSourceLabels:  labels,
		DataSourceBaseUrl: parsedUrl,
		dataSourceNames:   sources,
		client:            cl,
		pod:               pod,
		namespace:         "default",
	}
}

func (s *ControllerNotifierTestSuite) makeBitflowSample() *bitflow.Sample {
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

func (s *ControllerNotifierTestSuite) assertSources(cl client.Client, count int) {
	var sourceList bitflowv1.BitflowSourceList
	s.NoError(cl.List(context.TODO(), &client.ListOptions{}, &sourceList))
	s.Len(sourceList.Items, count)
}
