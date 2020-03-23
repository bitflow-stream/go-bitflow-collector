package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/antongulenko/golib"
	"github.com/bitflow-stream/go-bitflow/bitflow"
	"github.com/bitflow-stream/go-bitflow/script/reg"
	"github.com/bitflow-stream/go-bitflow/steps"
	zeropsv1 "github.com/citlab/zerops-operator/pkg/apis/zerops/v1"
	"github.com/citlab/zerops-operator/pkg/common"
	"github.com/citlab/zerops-operator/pkg/request"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	controllerClient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	EnvOwnPodName        = "POD_NAME"
	obtainIpRetryTimeout = 5 * time.Second
)

// Kubernetes ZerOpsDataSource objects are created automatically when samples appear containing a previously unseen value
// for a configured tag. The DataSource objects are checked (and when necessary re-created) in intervals given by UpdateInterval.
type ZeropsDataSourceNotifier struct {
	steps.TagChangeListener
	Orchestrator string

	DataSourceName    string
	DataSourceLabels  map[string]string
	DataSourceBaseUrl *url.URL

	dataSourceHost  string
	dataSourceNames map[string]string
	client          controllerClient.Client
	pod             *corev1.Pod
	namespace       string
}

func RegisterZeropsDataSourceNotifier(name string, b reg.ProcessorRegistry) {
	step := b.RegisterStep(name, func(p *bitflow.SamplePipeline, params map[string]interface{}) error {
		client, err := request.GetKubernetesClient(params["kube"].(string))
		if err != nil {
			return fmt.Errorf("Failed to initialize Kubernetes client: %v", err)
		}

		namespace := params["kube-namespace"].(string)
		// Get info on our own Pod
		podName := os.Getenv(EnvOwnPodName)
		var pod *corev1.Pod
		if podName == "" {
			log.Warnf("Environment variable %v is not set, cannot obtain own Pod. Managed %v will not have an ownerReference.",
				EnvOwnPodName, zeropsv1.DataSourcesKind)
		} else {
			log.Printf("Requesting information on own pod %v...", podName)
			pod, err = request.RequestPod(client, podName, namespace)
			if err != nil {
				return fmt.Errorf("Failed to query pod named %v: %v", podName, err)
			}
		}

		dataSourceUrl, err := url.Parse(params["dataSourceUrl"].(string))
		if err != nil {
			return reg.ParameterError("dataSourceUrl", err)
		}

		orchestrator := params["orchestrator"].(string)
		if orchestrator == "" && dataSourceUrl.Hostname() == "" {
			return reg.ParameterError("orchestrator", fmt.Errorf("The 'orchestrator' parameter is required, when the 'dataSourceUrl' parameter is given without a host name."))
		} else if orchestrator != "" && dataSourceUrl.Hostname() != "" {
			return reg.ParameterError("orchestrator", fmt.Errorf("When specifying the 'orchestrator' parameter, the 'dataSourceUrl' must not contain a host name."))
		}
		if dataSourceUrl.Path != "" || dataSourceUrl.RawQuery != "" {
			log.Warnf("The path and query of the dataSourceUrl will be ignored: %v", dataSourceUrl)
		}

		step := &ZeropsDataSourceNotifier{
			Orchestrator:      orchestrator,
			DataSourceName:    params["name"].(string),
			DataSourceLabels:  params["labels"].(map[string]string),
			DataSourceBaseUrl: dataSourceUrl,
			dataSourceNames:   make(map[string]string),
			client:            client,
			pod:               pod,
			namespace:         namespace,
		}
		step.TagChangeListener.Callback = step
		step.TagChangeListener.ReadParameters(params)
		p.Add(step)
		return nil
	}, "Notify the ZerOps controller about new tag values, where each tag value represents one stream of data").
		Optional("orchestrator", reg.String(), "").
		Optional("kube", reg.String(), "").
		Optional("kube-namespace", reg.String(), "").
		Required("name", reg.String()).
		Required("labels", reg.Map(reg.String())).
		Required("dataSourceUrl", reg.String())
	steps.AddTagChangeListenerParams(step)
}

func (t *ZeropsDataSourceNotifier) Start(wg *sync.WaitGroup) golib.StopChan {
	if t.Orchestrator != "" {
		t.obtainDataSourceHost()
	}
	return t.TagChangeListener.Start(wg)
}

func (t *ZeropsDataSourceNotifier) String() string {
	return fmt.Sprintf("%v: ZerOps data-source notifier. Data source labels: %v", t.TagChangeListener.String(), t.DataSourceLabels)
}

func (t *ZeropsDataSourceNotifier) Expired(value string, _ []string) bool {
	name, ok := t.dataSourceNames[value]
	if !ok {
		log.Warnf("The name of the data source for the tag value '%v' is not known, cannot delete it.", value)
		return true
	}
	source := &zeropsv1.ZerOpsDataSource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: t.namespace,
			Name:      name,
		},
	}
	return t.delete(source)
}

func (t *ZeropsDataSourceNotifier) delete(obj runtime.Object) bool {
	if err := t.client.Delete(context.TODO(), obj); err != nil {
		if errors.IsNotFound(err) {
			log.Debugf("Failed to delete %v: %v", obj, err)
			return true
		} else {
			log.Errorf("Failed to delete %v: %v", obj, err)
			return false
		}
	}
	return true
}

func (t *ZeropsDataSourceNotifier) Updated(value string, sample *bitflow.Sample, _ []string) {
	// Construct the data source and make sure it exists. Store the name so it can be deleted if necessary.
	source := t.makeDataSource(value, sample)
	sourceName, err := t.ensureDataSource(source)
	if err == nil {
		t.dataSourceNames[value] = sourceName
	} else {
		log.Errorf("Failed to create data source named %v: %v", source.Name, err)
	}
}

func (t *ZeropsDataSourceNotifier) obtainDataSourceHost() {
	log.Printf("Trying to contact ZerOps orchestrator to obtain data source IP...")
	for {
		ipString, err := readRequest(http.StatusOK)(http.Get("http://" + t.Orchestrator + "/ip"))
		if err != nil {
			log.Warnf("Failed to obtain own IP by contacting ZerOps orchestrator: %v", err)
			log.Warnf("Trying again in %v...", obtainIpRetryTimeout)
			time.Sleep(obtainIpRetryTimeout)
		} else {
			log.Printf("Obtained own IP from ZerOps orchestrator: %v", ipString)
			t.dataSourceHost = ipString
			break
		}
	}
}

func (t *ZeropsDataSourceNotifier) makeDataSource(tagVal string, sample *bitflow.Sample) *zeropsv1.ZerOpsDataSource {
	urlEndpoint := t.constructDataSourceEndpoint(tagVal)
	name := bitflow.ResolveTagTemplate(t.DataSourceName, "", sample)
	sourceLabels := make(map[string]string)
	for key, val := range t.DataSourceLabels {
		// Allow sample tag values to be places in the data source labels. Use the tags of the first sample that is part of the data stream. The tags should remain stable.
		sourceLabels[key] = bitflow.ResolveTagTemplate(val, "", sample)
	}
	name = common.HashName(name+"-", append(listKeysAndValues(sourceLabels), urlEndpoint)...)

	return &zeropsv1.ZerOpsDataSource{
		TypeMeta: metav1.TypeMeta{
			Kind:       zeropsv1.DataSourcesKind,
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       t.namespace,
			Labels:          sourceLabels,
			OwnerReferences: makeOwnerReferences(t.pod, false, false),
		},
		Spec: zeropsv1.ZerOpsDataSourceSpec{
			URL: urlEndpoint,
		},
	}
}

func listKeysAndValues(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(m)*2)
	for _, key := range keys {
		result = append(result, key, m[key])
	}
	return result
}

func makeOwnerReferences(pod *corev1.Pod, isController, blockOwnerDeletion bool) []metav1.OwnerReference {
	if pod == nil || pod.Name == "" || pod.UID == "" {
		return nil
	}
	// For some reason these meta fields are not set when querying the Pod
	apiVersion := pod.APIVersion
	if apiVersion == "" {
		apiVersion = "v1"
	}
	kind := pod.Kind
	if kind == "" {
		kind = "Pod"
	}
	return []metav1.OwnerReference{{
		APIVersion:         apiVersion,
		Kind:               kind,
		Name:               pod.Name,
		UID:                pod.UID,
		Controller:         &isController,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}}
}

func (t *ZeropsDataSourceNotifier) constructDataSourceEndpoint(val string) string {
	u := *t.DataSourceBaseUrl // Copy
	if t.dataSourceHost != "" {
		port := u.Port()
		if port != "" {
			port = ":" + port
		}
		u.Host = t.dataSourceHost + port
	}
	u.Path += "/" + val
	return u.String()
}

func (t *ZeropsDataSourceNotifier) ensureDataSource(source *zeropsv1.ZerOpsDataSource) (string, error) {
	// Query all data sources with exactly our labels (do not specify the name)
	selector := labels.SelectorFromSet(source.Labels)

	var list zeropsv1.ZerOpsDataSourceList
	err := t.client.List(context.TODO(), &controllerClient.ListOptions{LabelSelector: selector}, &list)
	if err != nil {
		return "", fmt.Errorf("Failed to query %v objects with labels: %v: %v", zeropsv1.DataSourcesKind, source.Labels, err)
	}

	existingName := ""
	for _, existingSource := range list.GetItems() {
		if len(existingSource.Labels) != len(source.Labels) {
			// Ignore sources that have a different number of labels. We are only interested in EXACTLY our labels. Unfortunately, it seems not possible to encode this directly in the List() request.
			continue
		}

		// Check if a data source with our properties already exists
		if source.EqualSpec(existingSource) {
			if existingName == "" {
				// We found our data source!
				existingName = existingSource.Name
				log.Debugf("%v already exists (named %v)", source, existingName)
				continue
			} else {
				// We already found an existing source, therefore fall through to cleaning up this one.
			}
		}

		// "Clean up" data sources that match our labels but do not match the rest of our spec.
		log.Printf("Deleting %v (either duplicate or does not match spec of %v)", existingSource, source)
		t.delete(existingSource)
	}

	if existingName == "" {
		log.Printf("Creating %v", source)
		if err = t.client.Create(context.TODO(), source); err != nil {
			err = fmt.Errorf("Failed to create %v: %v", source, err)
		} else {
			existingName = source.Name
		}
	}
	return existingName, err
}

func readRequest(expectedCode int) func(resp *http.Response, err error) (string, error) {
	return func(resp *http.Response, err error) (string, error) {
		var body string
		if err == nil {
			var bodyData []byte
			bodyData, err = ioutil.ReadAll(resp.Body)
			body = string(bodyData)
			if err != nil {
				err = fmt.Errorf("Failed to read response body: %v", err)
			}

			if resp.StatusCode != expectedCode {
				var bodyStr string
				if err == nil {
					bodyStr = "Body: " + body
				} else {
					bodyStr = err.Error()
				}
				err = fmt.Errorf("Reponse return status code %v. %v", resp.StatusCode, bodyStr)
			}
		}
		return body, err
	}
}
