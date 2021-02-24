package juicefs

import (
	"fmt"
	"io"
	"k8s.io/klog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

type metricsProxy struct {
	client           *http.Client
	nextMetricsPort  int
	mountMetricsPort map[string]int
}

func newMetricsProxy() *metricsProxy {
	return &metricsProxy{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		nextMetricsPort:  9567,
		mountMetricsPort: make(map[string]int),
	}
}

func (e *metricsProxy) serveMetricsHTTP(w http.ResponseWriter, req *http.Request) {
	wg := new(sync.WaitGroup)
	mfsCh := make(chan []*dto.MetricFamily)
	mfsResultCh := make(chan []*dto.MetricFamily)

	for key, port := range e.mountMetricsPort {
		wg.Add(1)
		go func() {
			defer wg.Done()
			endpoint := fmt.Sprintf("http://localhost:%d/metrics", port)
			metricFamilies, err := e.scrape(endpoint)
			if err != nil {
				klog.V(5).Infof("Scrape metrics from %s error: %q", endpoint, err)
				return
			}
			fields := strings.SplitN(key, ":", 2)
			labels := model.LabelSet{
				"volume_name": model.LabelValue(fields[0]),
				"mount_point": model.LabelValue(fields[1]),
			}
			rewriteMetrics(labels, metricFamilies)
			mfsCh <- metricFamilies
		}()
	}

	go func() {
		metricFamilies := make([]*dto.MetricFamily, 0)
		for mfs := range mfsCh {
			metricFamilies = append(metricFamilies, mfs...)
		}
		mfsResultCh <- metricFamilies
		close(mfsResultCh)
	}()

	klog.V(5).Infof("Waiting for scrape to return ...")
	wg.Wait()
	close(mfsCh)

	results := <-mfsResultCh
	contentType := expfmt.Negotiate(req.Header)
	encoder := expfmt.NewEncoder(w, contentType)
	for _, mf := range results {
		if err := encoder.Encode(mf); err != nil {
			http.Error(w, "An error has occurred during metrics encoding:\n\n"+err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (e *metricsProxy) scrape(address string) ([]*dto.MetricFamily, error) { // nolint: lll
	req, err := http.NewRequest("GET", address, nil)
	if err != nil {
		return nil, err
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned HTTP status %s", resp.Status)
	}

	return decodeMetrics(resp.Body, expfmt.ResponseFormat(resp.Header))
}

func decodeMetrics(reader io.Reader, format expfmt.Format) ([]*dto.MetricFamily, error) {
	metricFamilies := make([]*dto.MetricFamily, 0)
	decoder := expfmt.NewDecoder(reader, format)
	var err error
	for {
		mf := &dto.MetricFamily{}
		if err = decoder.Decode(mf); err == nil {
			metricFamilies = append(metricFamilies, mf)
		} else {
			break
		}
	}
	if err == io.EOF {
		err = nil
	}
	return metricFamilies, err
}

// rewriteMetrics adds the given LabelSet to all metrics in the given MetricFamily.
func rewriteMetrics(labels model.LabelSet, metricFamilies []*dto.MetricFamily) {
	for _, mf := range metricFamilies {
		for _, m := range mf.Metric {
			labelSet := make(model.LabelSet, len(m.Label))
			for _, lp := range m.Label {
				if lp.Name != nil {
					labelSet[model.LabelName(*lp.Name)] = model.LabelValue(lp.GetValue())
				}
			}
			mergedLabels := labelSet.Merge(labels)
			labelNames := make(model.LabelNames, 0, len(mergedLabels))
			for name := range mergedLabels {
				labelNames = append(labelNames, name)
			}
			sort.Sort(labelNames)
			labelPairs := make([]*dto.LabelPair, 0, len(mergedLabels))
			for _, name := range labelNames {
				labelPairs = append(labelPairs, &dto.LabelPair{
					// Note: could probably drop the function call and just pass a pointer
					Name:  proto.String(string(name)),
					Value: proto.String(string(mergedLabels[name])),
				})
			}
			m.Label = labelPairs
		}
	}
}
