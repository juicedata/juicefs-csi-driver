package sidecar

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

func TestEffectiveSidecarStateFallsBackToContainerCommand(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
		common.SidecarBinaryUpgradeAnnotationKey: `{"jfs-mount":{"image":"target","containerID":"containerd://current"}}`,
	}}, Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: "jfs-mount", ContainerID: "containerd://current"}}}}
	container := corev1.Container{
		Name: common.MountContainerName, Image: "original",
		Command: []string{"sh", "-c", `exec /usr/bin/juicefs mount volume /jfs/path -o 'foreground,no-update'`},
	}
	image, cmd, known, err := EffectiveSidecarState(pod, container)
	if err != nil {
		t.Fatal(err)
	}
	if image != "target" || !known || !reflect.DeepEqual(cmd, []string{"/usr/bin/juicefs", "mount", "volume", "/jfs/path", "-o", "foreground,no-update"}) {
		t.Fatalf("state = (%q, %#v, %v)", image, cmd, known)
	}

	annotatedCmd := []string{"/usr/bin/juicefs", "mount", "annotated", "/jfs/annotated"}
	state, err := json.Marshal(map[string]config.SidecarBinaryUpgradeInfo{
		container.Name: {ContainerID: "containerd://current", Image: "annotated-image", Cmd: annotatedCmd},
	})
	if err != nil {
		t.Fatal(err)
	}
	pod.Annotations[common.SidecarBinaryUpgradeAnnotationKey] = string(state)
	image, cmd, known, err = EffectiveSidecarState(pod, container)
	if err != nil {
		t.Fatal(err)
	}
	if image != "annotated-image" || !known || !reflect.DeepEqual(cmd, annotatedCmd) {
		t.Fatalf("annotation should take precedence: (%q, %#v, %v)", image, cmd, known)
	}

	pod.Annotations = nil
	image, cmd, known, err = EffectiveSidecarState(pod, container)
	if err != nil {
		t.Fatal(err)
	}
	if image != "original" || !known || len(cmd) == 0 {
		t.Fatalf("state without annotation = (%q, %#v, %v)", image, cmd, known)
	}

	container.Command = []string{"sh", "-c", "echo not-a-mount"}
	_, cmd, known, err = EffectiveSidecarState(pod, container)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != nil || known {
		t.Fatalf("malformed fallback = (%#v, %v), want unknown", cmd, known)
	}
}

func TestRequiredUpdates(t *testing.T) {
	target := TargetState{Image: "target", Cmd: []string{"mount", "vol", "/jfs"}}
	tests := []struct {
		name                 string
		currentImage         string
		currentCmd           []string
		cmdKnown, isCE       bool
		wantBinary, wantArgs bool
	}{
		{name: "unchanged", currentImage: "target", currentCmd: target.Cmd, cmdKnown: true},
		{name: "image changed", currentImage: "old", currentCmd: target.Cmd, cmdKnown: true, wantBinary: true},
		{name: "command changed", currentImage: "target", currentCmd: []string{"mount", "vol", "/old"}, cmdKnown: true, wantArgs: true},
		{name: "unknown command is conservative", currentImage: "target", wantArgs: true},
		{name: "both changed", currentImage: "old", currentCmd: []string{"mount", "vol", "/old"}, cmdKnown: true, wantBinary: true, wantArgs: true},
		{name: "CE ignores command", currentImage: "target", isCE: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetForCase := target
			targetForCase.IsCE = tt.isCE
			binary, args := RequiredUpdates(tt.currentImage, tt.currentCmd, tt.cmdKnown, targetForCase)
			if binary != tt.wantBinary || args != tt.wantArgs {
				t.Fatalf("RequiredUpdates() = (%v, %v), want (%v, %v)", binary, args, tt.wantBinary, tt.wantArgs)
			}
		})
	}
}

func TestResolveTargetStateCEKeepsBinaryOnly(t *testing.T) {
	oldConfig := config.GlobalConfig
	config.GlobalConfig = &config.Config{MountPodPatch: []config.MountPodPatch{{CEMountImage: "juicedata/mount:ce-target"}}}
	t.Cleanup(func() { config.GlobalConfig = oldConfig })
	setting := config.JfsSetting{
		IsCe: true, Name: "volume", MetaUrl: "redis://redis:6379/1", VolumeId: "volume-id", UniqueId: "unique-id", UUID: "uuid",
		Envs: map[string]string{}, Configs: map[string]string{},
		Attr: &config.PodAttr{Namespace: "default", Image: "juicedata/mount:ce-old", Labels: map[string]string{}, Annotations: map[string]string{}},
	}
	raw, _ := json.Marshal(setting)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default", Labels: map[string]string{common.InjectSidecarDone: common.True}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: common.MountContainerName, Command: []string{"sh", "-c", "exec /bin/mount.juicefs ${metaurl} /jfs/actual -o foreground"}}}},
	}
	got, err := ResolveTargetState(TargetStateRequest{Pod: pod, ContainerName: common.MountContainerName, SettingsSecret: &corev1.Secret{Data: map[string][]byte{"jfsSettings": raw}}})
	if err != nil {
		t.Fatalf("ResolveTargetState() error = %v", err)
	}
	if !got.IsCE || got.Image != "juicedata/mount:ce-target" || got.Cmd != nil {
		t.Fatalf("state = %#v, want CE binary-only target", got)
	}
}

func TestResolveTargetStateRendersExistingBuilderWithActualMountPath(t *testing.T) {
	oldConfig := config.GlobalConfig
	config.GlobalConfig = &config.Config{MountPodPatch: []config.MountPodPatch{{EEMountImage: "juicedata/mount:target"}}}
	t.Cleanup(func() { config.GlobalConfig = oldConfig })

	setting := config.JfsSetting{
		Name: "volume", Source: "volume", VolumeId: "volume-id", UniqueId: "unique-id", UUID: "uuid",
		MountPath: "/jfs/stale", Envs: map[string]string{}, Configs: map[string]string{},
		Attr: &config.PodAttr{Namespace: "default", Image: "juicedata/mount:old", Labels: map[string]string{}, Annotations: map[string]string{}},
	}
	raw, err := json.Marshal(setting)
	if err != nil {
		t.Fatal(err)
	}
	settingsSecret := &corev1.Secret{Data: map[string][]byte{"jfsSettings": raw}}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default", Labels: map[string]string{common.InjectSidecarDone: common.True}},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: common.MountContainerName, Image: "juicedata/mount:old",
			Command: []string{"sh", "-c", "exec /sbin/mount.juicefs volume /jfs/actual -o foreground,no-update"},
		}}},
	}
	pvc := &corev1.PersistentVolumeClaim{Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}}
	pv := &corev1.PersistentVolume{Spec: corev1.PersistentVolumeSpec{MountOptions: []string{"cache-size=123"}}}

	got, err := ResolveTargetState(TargetStateRequest{Pod: pod, ContainerName: common.MountContainerName, PVC: pvc, PV: pv, SettingsSecret: settingsSecret})
	if err != nil {
		t.Fatalf("ResolveTargetState() error = %v", err)
	}
	if got.Image != "juicedata/mount:target" {
		t.Fatalf("image = %q", got.Image)
	}
	want := []string{"/sbin/mount.juicefs", "volume", "/jfs/actual", "-o", "foreground,no-update,cache-size=123"}
	if !reflect.DeepEqual(got.Cmd, want) {
		t.Fatalf("cmd = %#v, want %#v", got.Cmd, want)
	}
}

func TestResolveTargetStateFromObjectsReportsMissingReferences(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name:         common.MountContainerName,
			VolumeMounts: []corev1.VolumeMount{{Name: "jfs-scripts", MountPath: "/jfs-scripts"}},
		}}, Volumes: []corev1.Volume{{
			Name: "jfs-scripts", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "settings"}},
		}}},
	}
	settings := corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name: "settings", Namespace: "default",
		OwnerReferences: []metav1.OwnerReference{{Kind: "PersistentVolumeClaim", Name: "pvc-1"}},
	}}
	base := TargetResources{Secrets: map[types.NamespacedName]corev1.Secret{{Namespace: "default", Name: "settings"}: settings}}

	if _, err := ResolveTargetStateFromObjects(pod, common.MountContainerName, base); err == nil || !strings.Contains(err.Error(), "PVC default/pvc-1") {
		t.Fatalf("missing PVC error = %v", err)
	}
	base.PVCs = map[string]corev1.PersistentVolumeClaim{"pvc-1": {ObjectMeta: metav1.ObjectMeta{Name: "pvc-1"}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "pv-1"}}}
	if _, err := ResolveTargetStateFromObjects(pod, common.MountContainerName, base); err == nil || !strings.Contains(err.Error(), "PV pv-1") {
		t.Fatalf("missing PV error = %v", err)
	}
	base.PVs = map[string]corev1.PersistentVolume{"pv-1": {
		ObjectMeta: metav1.ObjectMeta{Name: "pv-1"},
		Spec: corev1.PersistentVolumeSpec{PersistentVolumeSource: corev1.PersistentVolumeSource{CSI: &corev1.CSIPersistentVolumeSource{
			NodePublishSecretRef: &corev1.SecretReference{Namespace: "juicefs", Name: "customer"},
		}}},
	}}
	if _, err := ResolveTargetStateFromObjects(pod, common.MountContainerName, base); err == nil || !strings.Contains(err.Error(), "customer secret juicefs/customer") {
		t.Fatalf("missing customer Secret error = %v", err)
	}
}

func TestResolveUpgradeTargetsReturnsIdentityOnly(t *testing.T) {
	oldConfig := config.GlobalConfig
	config.GlobalConfig = &config.Config{MountPodPatch: []config.MountPodPatch{{EEMountImage: "juicedata/mount:target"}}}
	t.Cleanup(func() { config.GlobalConfig = oldConfig })

	setting := config.JfsSetting{
		Name: "volume", Source: "volume", VolumeId: "volume-id", UniqueId: "unique-id", UUID: "uuid",
		Envs: map[string]string{}, Configs: map[string]string{},
		Attr: &config.PodAttr{Namespace: "default", Image: "juicedata/mount:old", Labels: map[string]string{}, Annotations: map[string]string{}},
	}
	raw, err := json.Marshal(setting)
	if err != nil {
		t.Fatal(err)
	}
	settingsSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "jfs-secret", OwnerReferences: []metav1.OwnerReference{{Kind: "PersistentVolumeClaim", Name: "pvc-1"}}}, Data: map[string][]byte{"jfsSettings": raw}}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app", Namespace: "default", UID: types.UID("uid-1"),
			Labels: map[string]string{common.InjectSidecarDone: common.True},
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{{
				Name:         "jfs-scripts",
				VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "jfs-secret"}},
			}},
			Containers: []corev1.Container{{
				Name: common.MountContainerName, Image: "juicedata/mount:old",
				Command:      []string{"sh", "-c", "exec /sbin/mount.juicefs volume /jfs/actual -o foreground,no-update"},
				VolumeMounts: []corev1.VolumeMount{{Name: "jfs-scripts", MountPath: "/jfs-scripts"}},
			}},
		},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-1"},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: "pv-1",
			Resources:  corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
		},
	}
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-1"},
		Spec:       corev1.PersistentVolumeSpec{MountOptions: []string{"cache-size=123"}},
	}

	resources := TargetResources{
		PVCs:    map[string]corev1.PersistentVolumeClaim{"pvc-1": *pvc},
		PVs:     map[string]corev1.PersistentVolume{"pv-1": *pv},
		Secrets: map[types.NamespacedName]corev1.Secret{{Namespace: "default", Name: "jfs-secret"}: *settingsSecret},
	}
	eligible, skipped, err := ResolveUpgradeTargets(pod, []string{common.MountContainerName}, resources)
	if err != nil {
		t.Fatalf("ResolveUpgradeTargets() error = %v", err)
	}
	if len(eligible) != 1 || len(skipped) != 0 {
		t.Fatalf("eligible=%d skipped=%d, want 1/0", len(eligible), len(skipped))
	}
	tgt := eligible[0]
	if tgt.Name != "app" || tgt.ContainerName != common.MountContainerName || tgt.PodUID != types.UID("uid-1") {
		t.Fatalf("unexpected target identity: %#v", tgt)
	}
}

func TestResolveUpgradeTargetsFiltersUnsupportedArgumentOnlyUpdates(t *testing.T) {
	oldConfig := config.GlobalConfig
	t.Cleanup(func() { config.GlobalConfig = oldConfig })

	targetCmd := []string{"/sbin/mount.juicefs", "volume", "/jfs/actual", "-o", "foreground,no-update"}
	tests := []struct {
		name           string
		isCE           bool
		originalImage  string
		effectiveImage string
		effectiveCmd   []string
		wantEligible   int
		wantSkipped    int
	}{
		{
			name:           "command only uses original unsupported supervisor image",
			originalImage:  "juicedata/mount:ee-5.3.13",
			effectiveImage: "juicedata/mount:ee-5.4.3",
			effectiveCmd:   []string{"stale"},
			wantSkipped:    1,
		},
		{
			name:           "command only supports minimum supervisor image",
			originalImage:  "juicedata/mount:ee-5.4.3",
			effectiveImage: "juicedata/mount:ee-5.4.3",
			effectiveCmd:   []string{"stale"},
			wantEligible:   1,
		},
		{
			name:           "image only is eligible with old supervisor image",
			originalImage:  "juicedata/mount:ee-5.3.13",
			effectiveImage: "juicedata/mount:ee-5.3.13",
			effectiveCmd:   targetCmd,
			wantEligible:   1,
		},
		{
			name:           "image and command is eligible with old supervisor image",
			originalImage:  "juicedata/mount:ee-5.3.13",
			effectiveImage: "juicedata/mount:ee-5.3.13",
			effectiveCmd:   []string{"stale"},
			wantEligible:   1,
		},
		{
			name:           "CE still ignores command changes",
			isCE:           true,
			originalImage:  "juicedata/mount:ce-v1.2.0",
			effectiveImage: "juicedata/mount:ce-v1.2.0",
			effectiveCmd:   []string{"stale"},
			wantSkipped:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetImage := "juicedata/mount:ee-5.4.3"
			setting := config.JfsSetting{
				Name: "volume", Source: "volume", VolumeId: "volume-id", UniqueId: "unique-id", UUID: "uuid",
				Envs: map[string]string{}, Configs: map[string]string{},
				Attr: &config.PodAttr{Namespace: "default", Image: tt.originalImage, Labels: map[string]string{}, Annotations: map[string]string{}},
			}
			containerCommand := []string{"sh", "-c", "exec /sbin/mount.juicefs volume /jfs/actual -o foreground,no-update"}
			if tt.isCE {
				targetImage = "juicedata/mount:ce-v1.2.0"
				setting.IsCe = true
				setting.MetaUrl = "redis://redis:6379/1"
				containerCommand = []string{"sh", "-c", "exec /bin/mount.juicefs ${metaurl} /jfs/actual -o foreground"}
				config.GlobalConfig = &config.Config{MountPodPatch: []config.MountPodPatch{{CEMountImage: targetImage}}}
			} else {
				config.GlobalConfig = &config.Config{MountPodPatch: []config.MountPodPatch{{EEMountImage: targetImage}}}
			}
			raw, err := json.Marshal(setting)
			if err != nil {
				t.Fatal(err)
			}
			annotation, err := json.Marshal(map[string]config.SidecarBinaryUpgradeInfo{
				common.MountContainerName: {Image: tt.effectiveImage, Cmd: tt.effectiveCmd, ContainerID: "containerd://current"},
			})
			if err != nil {
				t.Fatal(err)
			}
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app", Namespace: "default", UID: types.UID("uid-1"),
					Labels:      map[string]string{common.InjectSidecarDone: common.True},
					Annotations: map[string]string{common.SidecarBinaryUpgradeAnnotationKey: string(annotation)},
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						Name: "jfs-scripts", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "jfs-secret"}},
					}},
					Containers: []corev1.Container{{
						Name: common.MountContainerName, Image: tt.originalImage, Command: containerCommand,
						VolumeMounts: []corev1.VolumeMount{{Name: "jfs-scripts", MountPath: "/jfs-scripts"}},
					}},
				},
				Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: common.MountContainerName, ContainerID: "containerd://current"}}},
			}
			settingsSecret := corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "jfs-secret", Namespace: "default"}, Data: map[string][]byte{"jfsSettings": raw}}
			resources := TargetResources{Secrets: map[types.NamespacedName]corev1.Secret{{Namespace: "default", Name: "jfs-secret"}: settingsSecret}}

			eligible, skipped, err := ResolveUpgradeTargets(pod, []string{common.MountContainerName}, resources)
			if err != nil {
				t.Fatalf("ResolveUpgradeTargets() error = %v", err)
			}
			if len(eligible) != tt.wantEligible || len(skipped) != tt.wantSkipped {
				t.Fatalf("eligible=%d skipped=%d, want %d/%d", len(eligible), len(skipped), tt.wantEligible, tt.wantSkipped)
			}
		})
	}
}

func TestResolveUpgradeTargetsMissingSettingsSecretSkipsTarget(t *testing.T) {
	oldConfig := config.GlobalConfig
	config.GlobalConfig = &config.Config{MountPodPatch: []config.MountPodPatch{{EEMountImage: "juicedata/mount:target"}}}
	t.Cleanup(func() { config.GlobalConfig = oldConfig })

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default", Labels: map[string]string{common.InjectSidecarDone: common.True}},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: common.MountContainerName, Image: "juicedata/mount:old",
			Command:      []string{"sh", "-c", "exec /sbin/mount.juicefs volume /jfs/actual -o foreground,no-update"},
			VolumeMounts: []corev1.VolumeMount{{Name: "jfs-scripts", MountPath: "/jfs-scripts"}},
		}}, Volumes: []corev1.Volume{{
			Name:         "jfs-scripts",
			VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "jfs-secret"}},
		}}},
	}
	eligible, skipped, err := ResolveUpgradeTargets(pod, []string{common.MountContainerName}, TargetResources{})
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if len(eligible) != 0 || len(skipped) != 0 {
		t.Fatalf("expected no targets, got eligible=%d skipped=%d", len(eligible), len(skipped))
	}
}
