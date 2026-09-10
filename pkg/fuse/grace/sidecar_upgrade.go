/*
Copyright 2024 Juicedata Inc

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

package grace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"path"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount/builder"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

const (
	acsLabelKey          = "alibabacloud.com/acs"
	acsComputeClassLabel = "alibabacloud.com/compute-class"
	acsComputeQOSLabel   = "alibabacloud.com/compute-qos"
)

type SidecarUpgradeTarget struct {
	Namespace     string
	PodName       string
	ContainerName string
}

type SidecarUpgradeRunner struct {
	*GraceUpgrade
	client        *k8s.K8sClient
	target        SidecarUpgradeTarget
	pod           *corev1.Pod
	confPath      string
	isCe          bool
	targetImage   string
	targetVersion string
	skipped       bool
	onFail        func()
}

var _ GraceRunner = &SidecarUpgradeRunner{}

func NewSidecarUpgradeRunner(client *k8s.K8sClient, target SidecarUpgradeTarget, pod *corev1.Pod, phaseTimeout time.Duration) *SidecarUpgradeRunner {
	return &SidecarUpgradeRunner{
		GraceUpgrade: &GraceUpgrade{client: client, phaseTimeout: phaseTimeout},
		client:       client,
		target:       target,
		pod:          pod,
	}
}

func RunSidecarUpgrade(ctx context.Context, client *k8s.K8sClient, target SidecarUpgradeTarget, phaseTimeout time.Duration) (bool, error) {
	runner := NewSidecarUpgradeRunner(client, target, nil, phaseTimeout)
	if err := runner.run(ctx, nil); err != nil {
		return false, err
	}
	return !runner.skipped, nil
}

func (r *SidecarUpgradeRunner) run(ctx context.Context, conn net.Conn) error {
	return r.GraceUpgrade.runGracefulUpgrade(ctx, r, conn)
}

func (r *SidecarUpgradeRunner) StatusPrefix() string {
	return "POD"
}

func (r *SidecarUpgradeRunner) TargetName() string {
	if r.target.PodName != "" {
		if r.target.ContainerName != "" {
			return r.target.PodName + "/" + r.target.ContainerName
		}
		return r.target.PodName
	}
	return r.target.ContainerName
}

func (r *SidecarUpgradeRunner) LockKey() string {
	return r.target.PodName
}

func (r *SidecarUpgradeRunner) PrepareShutdown(ctx context.Context) (*util.JuiceConf, error) {
	if r.pod == nil {
		pod, err := r.client.GetPod(ctx, r.target.PodName, r.target.Namespace)
		if err != nil {
			return nil, err
		}
		r.pod = pod
	}
	container, err := findSidecarContainer(r.pod, r.target.ContainerName)
	if err != nil {
		return nil, err
	}
	if err := config.LoadFromConfigMap(ctx, r.client); err != nil {
		return nil, err
	}
	r.targetImage, r.isCe, err = config.ResolveSidecarTargetImage(r.pod, container,
		func(name, namespace string) (*corev1.Secret, error) {
			return r.client.GetSecret(ctx, name, namespace)
		},
		func(name, namespace string) (*corev1.PersistentVolumeClaim, error) {
			return r.client.GetPersistentVolumeClaim(ctx, name, namespace)
		},
	)
	if err != nil {
		return nil, err
	}
	if r.targetImage == "" {
		r.sendMessage(fmt.Sprintf("POD-SKIP [%s/%s] target image is empty.", r.pod.Name, r.target.ContainerName))
		r.skipped = true
		return nil, nil
	}
	mntPath, _, err := util.GetMountPathOfSidecar(*r.pod, r.target.ContainerName)
	if err != nil {
		return nil, err
	}
	confName := util.GetJfsInternalFileNameOfContainer(container, ".config")
	r.confPath = path.Join(mntPath, confName)
	confContent, stderr, err := r.client.ExecuteInContainer(ctx, r.pod.Name, r.pod.Namespace, r.target.ContainerName, []string{"cat", r.confPath})
	if err != nil {
		return nil, fmt.Errorf("read %s failed: %v, stderr: %s", r.confPath, err, stderr)
	}
	jfsConf, err := util.ParseConfig([]byte(confContent))
	if err != nil {
		return nil, fmt.Errorf("parse %s failed: %w", r.confPath, err)
	}

	ownerReferences, err := resource.GetUpgradeJobOwnerReferences(ctx, r.client)
	if err != nil {
		return nil, err
	}
	labels, annotations, nodeSelector, tolerations := sidecarCanaryScheduling(r.pod)
	job := builder.NewCanaryJobFromSpec(builder.CanaryJobSpec{
		Name:            sidecarCanaryJobName(r.target),
		Namespace:       config.Namespace,
		Image:           r.targetImage,
		NodeSelector:    nodeSelector,
		Command:         []string{"sh", "-c", "sleep 300"},
		Labels:          labels,
		Annotations:     annotations,
		Tolerations:     tolerations,
		OwnerReferences: ownerReferences,
	})

	r.sendMessage(fmt.Sprintf("create canary job %s for %s/%s", job.Name, r.target.PodName, r.target.ContainerName))

	if _, err := r.client.CreateJob(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, err
	}
	r.sendMessage(fmt.Sprintf("wait for canary job %s for %s/%s running", job.Name, r.target.PodName, r.target.ContainerName))
	canaryPod, err := waitForCanaryPodRunning(ctx, r.client, job.Name)
	if err != nil {
		return nil, fmt.Errorf("fail to wait for canary job running: %w", err)
	}
	r.targetVersion = getCanaryBinaryVersion(ctx, r.client, canaryPod.Name, r.isCe)
	if r.targetVersion != "" {
		r.sendMessage(fmt.Sprintf("target version of %s/%s is %s", r.target.PodName, r.target.ContainerName, r.targetVersion))
	}
	if err := copySidecarBinary(ctx, r.client, canaryPod.Name, r.pod, r.target.ContainerName, r.isCe); err != nil {
		return nil, err
	}
	if err := r.GraceUpgrade.uploadBinary(ctx, r.pod, r.target.ContainerName, r.isCe); err != nil {
		return nil, err
	}
	if err := r.client.DeleteJob(ctx, job.Name, config.Namespace); err != nil {
		return nil, fmt.Errorf("delete canary job %s: %w", job.Name, err)
	}

	return jfsConf, nil
}

func sidecarCanaryScheduling(pod *corev1.Pod) (map[string]string, map[string]string, map[string]string, []corev1.Toleration) {
	if pod == nil {
		return nil, nil, nil, nil
	}

	labels := make(map[string]string, 3)
	if pod.Labels[acsLabelKey] == "true" {
		for _, key := range []string{acsLabelKey, acsComputeClassLabel, acsComputeQOSLabel} {
			if value, ok := pod.Labels[key]; ok {
				labels[key] = value
			}
		}
	}
	if pod.Labels[builder.CCIANNOKey] == builder.CCIANNOValue {
		labels[builder.CCIANNOKey] = builder.CCIANNOValue
	}

	var annotations map[string]string
	if pod.Annotations[builder.VCIANNOKey] == builder.VCIANNOValue {
		annotations = map[string]string{
			builder.VCIANNOKey: builder.VCIANNOValue,
		}
	}
	return labels, annotations, pod.Spec.NodeSelector, pod.Spec.Tolerations
}

func (r *SidecarUpgradeRunner) Sighup(ctx context.Context, jfsConf *util.JuiceConf) error {
	r.sendMessage(fmt.Sprintf("send SIGHUP to sidecar %s/%s/%s", r.target.Namespace, r.target.PodName, r.target.ContainerName))
	sighupAt := metav1.NewTime(time.Now().Add(-time.Second))
	if err := r.GraceUpgrade.sighup(ctx, r.pod, r.target.ContainerName, jfsConf.Pid); err != nil {
		return err
	}
	r.sendMessage(fmt.Sprintf("wait for sidecar %s/%s/%s to restart", r.target.Namespace, r.target.PodName, r.target.ContainerName))
	if err := waitForSidecarRestart(ctx, r.client, r.target, sighupAt, r.targetVersion); err != nil {
		return err
	}
	if err := updateSidecarUpgradeAnnotation(ctx, r.client, r.target, r.targetImage); err != nil {
		return err
	}
	upgradeEvtMsg := fmt.Sprintf("[%s] Upgrade binary in %s", r.pod.Name, r.target.ContainerName)
	if err := r.client.CreateEvent(ctx, *r.pod, corev1.EventTypeNormal, "Upgrade", upgradeEvtMsg); err != nil {
		log.Error(err, "fail to create event")
	}
	return nil
}

func (r *SidecarUpgradeRunner) OnFail() {
	if r.onFail != nil {
		r.onFail()
	}
}

func findSidecarContainer(pod *corev1.Pod, name string) (*corev1.Container, error) {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == name {
			return &pod.Spec.Containers[i], nil
		}
	}
	for i := range pod.Spec.InitContainers {
		if pod.Spec.InitContainers[i].Name == name {
			return &pod.Spec.InitContainers[i], nil
		}
	}
	return nil, fmt.Errorf("sidecar container %s not found in pod %s/%s", name, pod.Namespace, pod.Name)
}

func sidecarCanaryJobName(target SidecarUpgradeTarget) string {
	base := target.PodName + "-" + target.ContainerName
	base = base + "-" + util.RandStringRunes(6)
	return builder.GenJobNameByVolumeId(base) + "-canary"
}

func sidecarBinaryTarCommand(isCe bool) string {
	if isCe {
		return "tar cf - -C /usr/local/bin juicefs"
	}
	return "tar cf - -C /usr/bin juicefs -C /usr/local/juicefs/mount jfsmount"
}

func updateSidecarUpgradeAnnotation(ctx context.Context, client *k8s.K8sClient, target SidecarUpgradeTarget, targetImage string) error {
	pod, err := client.GetPod(ctx, target.PodName, target.Namespace)
	if err != nil {
		return err
	}
	annotations := config.ParseSidecarBinaryUpgradeAnnotation(pod)
	annotations[target.ContainerName] = config.SidecarBinaryUpgradeInfo{
		Image:      targetImage,
		UpgradedAt: metav1.NewTime(time.Now()),
	}
	raw, err := json.Marshal(annotations)
	if err != nil {
		return err
	}
	return resource.AddPodAnnotation(ctx, client, pod.Name, pod.Namespace, map[string]string{
		common.SidecarBinaryUpgradeAnnotationKey: string(raw),
	})
}

func waitForCanaryPodRunning(ctx context.Context, client *k8s.K8sClient, jobName string) (*corev1.Pod, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{common.CanaryJobLabelKey: jobName},
	}
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for canary job %s to run", jobName)
		case <-ticker.C:
			pods, err := client.ListPod(ctx, config.Namespace, &labelSelector, nil)
			if err != nil {
				return nil, err
			}
			for i := range pods {
				if resource.IsPodReady(&pods[i]) {
					return &pods[i], nil
				}
				if resource.IsPodError(&pods[i]) {
					jobErr := fmt.Errorf("canary pod %s failed, status: %s", pods[i].Name, resource.GetPodStatus(&pods[i]))
					return nil, resource.GetCanaryJobFailure(ctx, client, config.Namespace, jobName, "", jobErr)
				}
			}
		}
	}
}

func copySidecarBinary(ctx context.Context, client *k8s.K8sClient, canaryPodName string, sidecarPod *corev1.Pod, sidecarContainer string, isCe bool) error {
	reader, writer := io.Pipe()
	sourceErr := make(chan error, 1)
	go func() {
		err := client.ExecuteInContainerStream(ctx, canaryPodName, config.Namespace, "canary",
			[]string{"sh", "-c", sidecarBinaryTarCommand(isCe)}, nil, writer, nil)
		_ = writer.CloseWithError(err)
		sourceErr <- err
	}()

	destinationErr := client.ExecuteInContainerStream(ctx, sidecarPod.Name, sidecarPod.Namespace, sidecarContainer,
		[]string{"tar", "xf", "-", "-C", "/tmp"}, reader, nil, nil)
	if destinationErr != nil {
		_ = reader.CloseWithError(destinationErr)
	}
	if err := <-sourceErr; err != nil {
		return fmt.Errorf("copy binary from canary pod %s: %w", canaryPodName, err)
	}
	if destinationErr != nil {
		return fmt.Errorf("copy binary to sidecar pod %s/%s: %w", sidecarPod.Namespace, sidecarPod.Name, destinationErr)
	}
	return nil
}

const (
	sidecarRestartCheckInterval = 2 * time.Second
	sidecarRestartedMarker      = "JuiceFS version"
	sidecarFuseBusyMarker       = "FUSE session is busy, don't restart"
	sidecarSighupMarker         = "signal hangup"
)

// truncateBeforeLastSighup keeps only the log content produced after the last
// SIGHUP so that stale restart records cannot be mistaken for the current one.
func truncateBeforeLastSighup(logs string) string {
	lines := strings.Split(logs, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], sidecarSighupMarker) {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	return logs
}

var juicefsVersionRegex = regexp.MustCompile(`(?i)juicefs version\s+(\S+)(?:\s+\(([^)]*)\))?`)

func canaryVersionCommand(isCe bool) string {
	if isCe {
		return config.CeCliPath + " --version"
	}
	return config.CliPath + " --version"
}

// parseJuiceFSVersion extracts the version from a `--version` output or a mount log
// line. The enterprise edition prints the build date and commit in parentheses, which
// is part of the version, while the community edition prints the platform there and
// must be dropped.
func parseJuiceFSVersion(s string) string {
	matches := juicefsVersionRegex.FindStringSubmatch(s)
	if len(matches) < 2 {
		return ""
	}
	version := strings.TrimSpace(matches[1])
	if len(matches) > 2 {
		if extra := strings.TrimSpace(matches[2]); extra != "" && !strings.Contains(extra, "/") {
			version += " (" + extra + ")"
		}
	}
	return version
}

// getCanaryBinaryVersion reads the target binary version from the canary pod so that
// the sidecar restart can be verified against it. Failures are not fatal.
func getCanaryBinaryVersion(ctx context.Context, client *k8s.K8sClient, canaryPodName string, isCe bool) string {
	stdout, stderr, err := client.ExecuteInContainer(ctx, canaryPodName, config.Namespace, "canary",
		[]string{"sh", "-c", canaryVersionCommand(isCe)})
	if err != nil {
		log.Info("failed to get version from canary pod", "pod", canaryPodName, "error", err, "stderr", stderr)
		return ""
	}
	version := parseJuiceFSVersion(stdout)
	if version == "" {
		log.Info("failed to parse version from canary pod output", "pod", canaryPodName, "output", stdout)
	}
	return version
}

// evaluateSidecarRestartLog reports whether the sidecar mount process has restarted
// with the expected version, or an error when the SIGHUP was refused.
func evaluateSidecarRestartLog(logs string, expectVersion string) (bool, error) {
	logs = truncateBeforeLastSighup(logs)
	if strings.Contains(logs, sidecarFuseBusyMarker) {
		return false, fmt.Errorf("mount process ignored SIGHUP: %s", sidecarFuseBusyMarker)
	}
	for _, line := range strings.Split(logs, "\n") {
		if !strings.Contains(line, sidecarRestartedMarker) {
			continue
		}
		if expectVersion == "" {
			return true, nil
		}
		if parseJuiceFSVersion(line) == expectVersion {
			return true, nil
		}
	}
	return false, nil
}

func waitForSidecarRestart(ctx context.Context, client *k8s.K8sClient, target SidecarUpgradeTarget, sighupAt metav1.Time, expectVersion string) error {
	if expectVersion == "" {
		log.Info("target binary version is unknown, verify sidecar restart by log marker only",
			"pod", target.PodName, "container", target.ContainerName)
	}
	ticker := time.NewTicker(sidecarRestartCheckInterval)
	defer ticker.Stop()

	var lastLogs string
	for {
		logs, err := client.CoreV1().Pods(target.Namespace).GetLogs(target.PodName, &corev1.PodLogOptions{
			Container: target.ContainerName,
			SinceTime: &sighupAt,
		}).DoRaw(ctx)
		if err == nil {
			lastLogs = string(logs)
			done, evalErr := evaluateSidecarRestartLog(lastLogs, expectVersion)
			if evalErr != nil {
				return fmt.Errorf("sidecar %s/%s/%s failed to restart: %w", target.Namespace, target.PodName, target.ContainerName, evalErr)
			}
			if done {
				return nil
			}
		} else {
			log.Info("failed to read sidecar logs", "pod", target.PodName, "container", target.ContainerName, "error", err)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for sidecar %s/%s/%s to restart with version %q; logs: %s",
				target.Namespace, target.PodName, target.ContainerName, expectVersion, strings.TrimSpace(lastLogs))
		case <-ticker.C:
		}
	}
}
