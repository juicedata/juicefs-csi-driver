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
	"bytes"
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
	"k8s.io/apimachinery/pkg/types"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount/builder"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/sidecar"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

func sidecarRestartArgsPath(supervisorPID int) string {
	return fmt.Sprintf("/tmp/jfs_restart_args.%d", supervisorPID)
}

func supportsSidecarRestartArgs(version string) bool {
	var major, minor, patch int
	if _, err := fmt.Sscanf(version, "%d.%d.%d", &major, &minor, &patch); err != nil {
		return false
	}
	return major > 5 || major == 5 && (minor > 4 || minor == 4 && patch >= 3)
}

func supportsSidecarRestartArgsForProcesses(targetVersion, supervisorVersion string) bool {
	return supportsSidecarRestartArgs(targetVersion) && supportsSidecarRestartArgs(supervisorVersion)
}

func marshalSidecarRestartArgs(args []string) ([]byte, error) {
	if len(args) < 2 || args[0] == "" {
		return nil, fmt.Errorf("invalid sidecar restart args")
	}
	return json.Marshal(args)
}

func restartArgsWriteCommand(supervisorPID int) string {
	final := sidecarRestartArgsPath(supervisorPID)
	return fmt.Sprintf("set -e; umask 077; tmp=%s.tmp.$$; trap 'rm -f \"$tmp\"' EXIT; cat >\"$tmp\"; chmod 600 \"$tmp\"; mv -f \"$tmp\" %s; trap - EXIT", final, final)
}

// parseProcCmdline parses the content of /proc/<pid>/cmdline. Once the jfs
// supervisor applies restart args it rewrites its own cmdline via setproctitle
// into a single space-joined title padded with NUL bytes, truncated to the
// original argv area when the new args are longer. Such a title is split back
// into words and reported as rewritten.
func parseProcCmdline(raw string) (argv []string, rewritten bool) {
	parts := strings.Split(raw, "\x00")
	for len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 1 && strings.Contains(parts[0], " ") {
		return strings.Fields(parts[0]), true
	}
	return parts, false
}

func validateRestartedCommand(oldPID, newPID int, actual, expected []string, actualRewritten bool) error {
	if newPID <= 0 || newPID == oldPID {
		return fmt.Errorf("mount worker PID did not change: %d", newPID)
	}
	if len(actual) < 2 || len(expected) < 2 {
		return fmt.Errorf("mount command is empty")
	}
	if len(actual) > len(expected) || (!actualRewritten && len(actual) != len(expected)) {
		return fmt.Errorf("mount argv length mismatch: got %d (actual %q), want %d (expected %q)", len(actual), actual, len(expected), expected)
	}
	for i := 1; i < len(actual); i++ {
		if actual[i] == expected[i] {
			continue
		}
		// A rewritten proc title cannot grow beyond the original argv area,
		// so its last visible word may be a prefix of the expected argument
		// and trailing arguments may be missing entirely.
		if actualRewritten && i == len(actual)-1 && strings.HasPrefix(expected[i], actual[i]) {
			continue
		}
		return fmt.Errorf("mount argv mismatch at %d: got %q, want %q", i, actual[i], expected[i])
	}
	return nil
}

const (
	acsLabelKey          = "alibabacloud.com/acs"
	acsComputeClassLabel = "alibabacloud.com/compute-class"
	acsComputeQOSLabel   = "alibabacloud.com/compute-qos"
)

type SidecarUpgradeTarget struct {
	Namespace     string
	PodName       string
	ContainerName string
	PodUID        types.UID
	ContainerID   string
}

type SidecarUpgradeRunner struct {
	*GraceUpgrade
	client         *k8s.K8sClient
	target         SidecarUpgradeTarget
	pod            *corev1.Pod
	confPath       string
	isCe           bool
	targetImage    string
	currentCmd     []string
	targetCmd      []string
	updateBinary   bool
	updateArgs     bool
	targetVersion  string
	mountPath      string
	supervisorPID  int
	binaryReplaced bool
	skipped        bool
	onFail         func()
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

func (r *SidecarUpgradeRunner) resolveTargetState(ctx context.Context, container *corev1.Container) (sidecar.TargetState, error) {
	settingsName := sidecar.SettingsSecretName(r.pod, container)
	if settingsName == "" {
		return sidecar.TargetState{}, sidecar.ErrSettingsSecretMissing
	}
	settingsSecret, err := r.client.GetSecret(ctx, settingsName, r.pod.Namespace)
	if err != nil {
		return sidecar.TargetState{}, err
	}
	var pvc *corev1.PersistentVolumeClaim
	for _, owner := range settingsSecret.OwnerReferences {
		if owner.Kind == "PersistentVolumeClaim" && owner.Name != "" {
			pvc, err = r.client.GetPersistentVolumeClaim(ctx, owner.Name, r.pod.Namespace)
			if err != nil {
				return sidecar.TargetState{}, err
			}
			break
		}
	}
	var pv *corev1.PersistentVolume
	if pvc != nil && pvc.Spec.VolumeName != "" {
		pv, err = r.client.GetPersistentVolume(ctx, pvc.Spec.VolumeName)
		if err != nil {
			return sidecar.TargetState{}, err
		}
	}
	var customerSecret *corev1.Secret
	if pv != nil && pv.Spec.CSI != nil && pv.Spec.CSI.NodePublishSecretRef != nil {
		ref := pv.Spec.CSI.NodePublishSecretRef
		customerSecret, err = r.client.GetSecret(ctx, ref.Name, ref.Namespace)
		if err != nil {
			return sidecar.TargetState{}, err
		}
	}
	var node *corev1.Node
	if r.pod.Spec.NodeName != "" {
		node, err = r.client.GetNode(ctx, r.pod.Spec.NodeName)
		if err != nil {
			return sidecar.TargetState{}, err
		}
	}
	return sidecar.ResolveTargetState(sidecar.TargetStateRequest{
		Pod: r.pod, ContainerName: r.target.ContainerName, PVC: pvc, PV: pv,
		SettingsSecret: settingsSecret, CustomerSecret: customerSecret, Node: node,
	})
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
	name := r.target.PodName
	if r.target.Namespace != "" && name != "" {
		name = r.target.Namespace + "/" + name
	}
	if r.target.PodName != "" {
		if r.target.ContainerName != "" {
			return name + "/" + r.target.ContainerName
		}
		return name
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
	if r.target.PodUID != "" && r.pod.UID != r.target.PodUID {
		return nil, fmt.Errorf("pod UID changed: got %q, want %q", r.pod.UID, r.target.PodUID)
	}
	if r.target.ContainerID != "" && currentContainerID(r.pod, r.target.ContainerName) != r.target.ContainerID {
		return nil, fmt.Errorf("sidecar container ID changed")
	}
	if err := config.LoadFromConfigMap(ctx, r.client); err != nil {
		return nil, err
	}
	desired, err := r.resolveTargetState(ctx, container)
	if err != nil {
		return nil, err
	}
	r.targetImage, r.targetCmd, r.isCe = desired.Image, desired.Cmd, desired.IsCE
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
	r.mountPath = mntPath
	currentImage, currentCmd, cmdKnown, err := sidecar.EffectiveSidecarState(r.pod, *container)
	if err != nil {
		return nil, err
	}
	if !r.isCe && !cmdKnown {
		currentCmd, _, err = r.readProcessCmdline(ctx, jfsConf.Pid)
		if err != nil {
			return nil, err
		}
		cmdKnown = true
	}
	r.currentCmd = currentCmd
	r.updateBinary, r.updateArgs = sidecar.RequiredUpdates(currentImage, currentCmd, cmdKnown, desired)
	if !r.updateBinary && !r.updateArgs {
		if err := updateSidecarUpgradeAnnotation(ctx, r.client, r.target, r.targetImage, r.targetCmd); err != nil {
			return nil, err
		}
		r.skipped = true
		return nil, nil
	}
	if r.updateArgs {
		if len(r.targetCmd) == 0 {
			return nil, fmt.Errorf("target mount command is empty")
		}
		r.supervisorPID, err = resolvePpid(jfsConf, r.pod.Spec.HostPID)
		if err != nil {
			return nil, err
		}
	}
	if !r.updateBinary {
		return jfsConf, nil
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
	r.binaryReplaced = true
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

func changedPodSpecMountOptions(currentCmd, targetCmd []string) []string {
	var changed []string
	for _, name := range []string{"cache-dir", "rsa-key"} {
		target := mountOptionValues(targetCmd, name)
		if len(target) > 0 && !util.EqualStringLists(mountOptionValues(currentCmd, name), target) {
			changed = append(changed, name)
		}
	}
	return changed
}

func mountOptionValues(cmd []string, name string) []string {
	for i, arg := range cmd {
		if arg != "-o" || i+1 >= len(cmd) {
			continue
		}
		var values []string
		for _, option := range strings.Split(cmd[i+1], ",") {
			if option == name || strings.HasPrefix(option, name+"=") {
				values = append(values, option)
			}
		}
		return values
	}
	return nil
}

func podSpecMountOptionsWarning(target SidecarUpgradeTarget, changed []string) string {
	return fmt.Sprintf("WARNING [%s/%s/%s] changed mount options %v require corresponding pod spec to take effect",
		target.Namespace, target.PodName, target.ContainerName, changed)
}

func mountArgumentSkipMessage(target SidecarUpgradeTarget, reason string, updateBinary bool) string {
	status := "POD-SKIP"
	if updateBinary {
		status = "WARNING"
	}
	return fmt.Sprintf("%s [%s/%s/%s] mount argument update skipped: %s",
		status, target.Namespace, target.PodName, target.ContainerName, reason)
}

func (r *SidecarUpgradeRunner) Sighup(ctx context.Context, jfsConf *util.JuiceConf) error {
	annotationCmd := r.targetCmd
	if r.updateArgs {
		version := r.targetVersion
		if version == "" && !r.updateBinary {
			stdout, stderr, err := r.client.ExecuteInContainer(ctx, r.pod.Name, r.pod.Namespace, r.target.ContainerName,
				[]string{"sh", "-c", canaryVersionCommand(r.isCe)})
			if err != nil {
				log.Info("failed to get sidecar version before updating mount arguments", "error", err, "stderr", stderr)
			} else {
				version = parseJuiceFSVersion(stdout)
				r.targetVersion = version
			}
		}
		if !supportsSidecarRestartArgs(version) {
			r.sendMessage(mountArgumentSkipMessage(r.target,
				fmt.Sprintf("image version %q is below 5.4.3 or unknown", version), r.updateBinary))
			r.updateArgs = false
			annotationCmd = nil
		} else {
			if r.supervisorPID == 0 {
				var err error
				r.supervisorPID, err = resolvePpid(jfsConf, r.pod.Spec.HostPID)
				if err != nil {
					return err
				}
			}
			supervisorVersion := r.readProcessVersion(ctx, r.supervisorPID)
			if !supportsSidecarRestartArgsForProcesses(version, supervisorVersion) {
				r.sendMessage(mountArgumentSkipMessage(r.target,
					fmt.Sprintf("supervisor version %q is below 5.4.3 or unknown; recreate the pod before updating mount arguments", supervisorVersion), r.updateBinary))
				r.updateArgs = false
				annotationCmd = nil
			}
		}
	}
	if !r.updateBinary && !r.updateArgs {
		if err := updateSidecarUpgradeAnnotation(ctx, r.client, r.target, r.targetImage, annotationCmd); err != nil {
			return err
		}
		r.skipped = true
		return nil
	}
	if r.updateArgs {
		if len(r.targetCmd) == 0 {
			return fmt.Errorf("target mount command is empty")
		}
		if changed := changedPodSpecMountOptions(r.currentCmd, r.targetCmd); len(changed) > 0 {
			r.sendMessage(podSpecMountOptionsWarning(r.target, changed))
		}
		if err := r.writeRestartArgs(ctx, r.supervisorPID, r.targetCmd); err != nil {
			return err
		}
		defer r.removeRestartArgs(context.WithoutCancel(ctx), r.supervisorPID)
	}

	r.sendMessage(fmt.Sprintf("send SIGHUP to sidecar %s/%s/%s", r.target.Namespace, r.target.PodName, r.target.ContainerName))
	sighupAt := metav1.NewTime(time.Now().Add(-time.Second))
	if err := r.GraceUpgrade.sighup(ctx, r.pod, r.target.ContainerName, jfsConf.Pid); err != nil {
		return err
	}
	r.sendMessage(fmt.Sprintf("wait for sidecar %s/%s/%s to restart", r.target.Namespace, r.target.PodName, r.target.ContainerName))
	if err := waitForSidecarRestart(ctx, r.client, r.target, sighupAt, r.targetVersion); err != nil {
		return err
	}
	if r.updateArgs {
		if err := r.verifyRestartedCommand(ctx, jfsConf.Pid, r.targetCmd); err != nil {
			return err
		}
		r.sendMessage(fmt.Sprintf("mount arguments of sidecar %s/%s/%s updated to %q",
			r.target.Namespace, r.target.PodName, r.target.ContainerName, r.targetCmd[1:]))
	}
	if r.isCe {
		annotationCmd = nil
	}
	if err := updateSidecarUpgradeAnnotation(ctx, r.client, r.target, r.targetImage, annotationCmd); err != nil {
		return err
	}
	upgradeEvtMsg := fmt.Sprintf("[%s] Upgrade binary in %s", r.pod.Name, r.target.ContainerName)
	if err := r.client.CreateEvent(ctx, *r.pod, corev1.EventTypeNormal, "Upgrade", upgradeEvtMsg); err != nil {
		log.Error(err, "fail to create event")
	}
	return nil
}

func (r *SidecarUpgradeRunner) readProcessVersion(ctx context.Context, pid int) string {
	stdout, stderr, err := r.client.ExecuteInContainer(ctx, r.pod.Name, r.pod.Namespace, r.target.ContainerName,
		[]string{fmt.Sprintf("/proc/%d/exe", pid), "-V"})
	if err != nil {
		log.Info("failed to get sidecar supervisor version before updating mount arguments", "pid", pid, "error", err, "stderr", stderr)
		return ""
	}
	version := parseJuiceFSVersion(stdout)
	if version == "" {
		log.Info("failed to parse sidecar supervisor version", "pid", pid, "output", stdout)
	}
	return version
}

func (r *SidecarUpgradeRunner) readProcessCmdline(ctx context.Context, pid int) ([]string, bool, error) {
	stdout, stderr, err := r.client.ExecuteInContainer(ctx, r.pod.Name, r.pod.Namespace, r.target.ContainerName,
		[]string{"cat", fmt.Sprintf("/proc/%d/cmdline", pid)})
	if err != nil {
		return nil, false, fmt.Errorf("read process %d cmdline: %w, stderr: %s", pid, err, stderr)
	}
	argv, rewritten := parseProcCmdline(strings.TrimSuffix(stdout, "\x00"))
	if len(argv) == 0 {
		return nil, false, fmt.Errorf("process %d cmdline is empty", pid)
	}
	return argv, rewritten, nil
}

func (r *SidecarUpgradeRunner) writeRestartArgs(ctx context.Context, supervisorPID int, args []string) error {
	payload, err := marshalSidecarRestartArgs(args)
	if err != nil {
		return err
	}
	if err := r.client.ExecuteInContainerStream(ctx, r.pod.Name, r.pod.Namespace, r.target.ContainerName,
		[]string{"sh", "-c", restartArgsWriteCommand(supervisorPID)}, bytes.NewReader(payload), nil, nil); err != nil {
		return fmt.Errorf("write sidecar restart args: %w", err)
	}
	return nil
}

func (r *SidecarUpgradeRunner) removeRestartArgs(ctx context.Context, supervisorPID int) {
	_, _, _ = r.client.ExecuteInContainer(ctx, r.pod.Name, r.pod.Namespace, r.target.ContainerName,
		[]string{"rm", "-f", sidecarRestartArgsPath(supervisorPID)})
}

func (r *SidecarUpgradeRunner) verifyRestartedCommand(ctx context.Context, oldPID int, expected []string) error {
	verify := func() error {
		content, stderr, err := r.client.ExecuteInContainer(ctx, r.pod.Name, r.pod.Namespace, r.target.ContainerName, []string{"cat", r.confPath})
		if err != nil {
			return fmt.Errorf("read restarted sidecar config: %w, stderr: %s", err, stderr)
		}
		conf, err := util.ParseConfig([]byte(content))
		if err != nil {
			return err
		}
		actual, actualRewritten, err := r.readProcessCmdline(ctx, conf.Pid)
		if err != nil {
			return err
		}
		if err := validateRestartedCommand(oldPID, conf.Pid, actual, expected, actualRewritten); err != nil {
			return err
		}
		inode, stderr, err := r.client.ExecuteInContainer(ctx, r.pod.Name, r.pod.Namespace, r.target.ContainerName,
			[]string{"timeout", "3", "stat", "-c", "%i", r.mountPath})
		if err != nil || strings.TrimSpace(inode) != "1" {
			return fmt.Errorf("mount path %s is not accessible as JuiceFS root: inode=%q, err=%v, stderr=%s", r.mountPath, strings.TrimSpace(inode), err, stderr)
		}
		return nil
	}
	if err := verify(); err != nil {
		return err
	}
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	return verify()
}

func (r *SidecarUpgradeRunner) OnFail() {
	if r.binaryReplaced && r.client != nil && r.pod != nil {
		script := "mv -f /tmp/juicefs.bak /usr/bin/juicefs; mv -f /tmp/jfsmount.bak /usr/local/juicefs/mount/jfsmount"
		if r.isCe {
			script = "mv -f /tmp/juicefs.bak /usr/local/bin/juicefs"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, stderr, err := r.client.ExecuteInContainer(ctx, r.pod.Name, r.pod.Namespace, r.target.ContainerName, []string{"sh", "-c", script}); err != nil {
			log.Error(err, "failed to roll back sidecar binary", "stderr", stderr)
		}
	}
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

func currentContainerID(pod *corev1.Pod, name string) string {
	if pod == nil {
		return ""
	}
	for _, status := range append(pod.Status.ContainerStatuses, pod.Status.InitContainerStatuses...) {
		if status.Name == name {
			return status.ContainerID
		}
	}
	return ""
}

func updateSidecarUpgradeAnnotation(ctx context.Context, client *k8s.K8sClient, target SidecarUpgradeTarget, targetImage string, targetCmd []string) error {
	for attempt := 0; attempt < 3; attempt++ {
		pod, err := client.GetPod(ctx, target.PodName, target.Namespace)
		if err != nil {
			return err
		}
		if target.PodUID != "" && pod.UID != target.PodUID {
			return fmt.Errorf("pod UID changed before recording upgrade")
		}
		containerID := currentContainerID(pod, target.ContainerName)
		if target.ContainerID != "" && containerID != target.ContainerID {
			return fmt.Errorf("sidecar container ID changed before recording upgrade")
		}
		annotations, err := config.ParseSidecarBinaryUpgradeAnnotation(pod)
		if err != nil {
			return err
		}
		annotations[target.ContainerName] = config.SidecarBinaryUpgradeInfo{
			Image: targetImage, Cmd: targetCmd, ContainerID: containerID,
			UpgradedAt: metav1.NewTime(time.Now()),
		}
		raw, err := json.Marshal(annotations)
		if err != nil {
			return err
		}
		patch, err := json.Marshal(map[string]interface{}{"metadata": map[string]interface{}{
			"resourceVersion": pod.ResourceVersion,
			"annotations":     map[string]string{common.SidecarBinaryUpgradeAnnotationKey: string(raw)},
		}})
		if err != nil {
			return err
		}
		err = client.PatchPod(ctx, pod.Name, pod.Namespace, patch, types.StrategicMergePatchType)
		if err == nil {
			return nil
		}
		if !apierrors.IsConflict(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 100 * time.Millisecond):
		}
	}
	return fmt.Errorf("update sidecar upgrade annotation: conflict retries exhausted")
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
