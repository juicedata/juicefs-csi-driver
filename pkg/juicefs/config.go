package juicefs

var (
	NodeName   = ""
	Namespace  = ""
	MountImage = ""

	MountPodCpuLimit   = "1"
	MountPodMemLimit   = "1G"
	MountPodCpuRequest = "1"
	MountPodMemRequest = "1G"

	MountPointPath = "/run/juicefs/volume"
)

const (
	VolumeId     = "juicefs.com/volume-id"
	PodTypeKey   = "app.kubernetes.io/name"
	PodTypeValue = "juicefs-mount"
	Finalizer    = "juicefs.com/finalizer"
)
