package juicefs

var (
	NodeName   = ""
	Namespace  = ""
	MountImage = ""

	MountPodCpuLimit   = "5000m"
	MountPodMemLimit   = "5Gi"
	MountPodCpuRequest = "1000m"
	MountPodMemRequest = "1Gi"

	MountPointPath = "/run/juicefs/volume"
)

const (
	VolumeId     = "juicefs.com/volume-id"
	PodTypeKey   = "app.kubernetes.io/name"
	PodTypeValue = "juicefs-mount"
	Finalizer    = "juicefs.com/finalizer"
)
